//go:build capdb

package store_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"capper/internal/store"
)

// capdbServer is a co-located capdb-server for integration tests. It can be
// killed and restarted on the same port + db-root to exercise R1 (self-healing
// pool) recovery.
type capdbServer struct {
	t       *testing.T
	bin     string
	dir     string
	auth    string
	addr    string
	token   string
	cmd     *exec.Cmd
}

func newCapdbServer(t *testing.T) *capdbServer {
	t.Helper()
	bin := os.Getenv("CAPDB_SERVER")
	if bin == "" {
		bin = filepath.FromSlash("../../build/capdb/capdb-server")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("capdb-server not built (%s); run `make capdb`", bin)
	}
	dir := t.TempDir()
	const token = "itest-token"
	auth := filepath.Join(dir, "auth.txt")
	if err := os.WriteFile(auth, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	s := &capdbServer{
		t: t, bin: bin, dir: dir, auth: auth, token: token,
		addr: fmt.Sprintf("127.0.0.1:%d", port),
	}
	s.start()
	t.Cleanup(s.stop)
	return s
}

func (s *capdbServer) start() {
	s.cmd = exec.Command(s.bin,
		"--insecure", "--listen", s.addr,
		"--auth-file", s.auth, "--db-root", s.dir, "--pool-max", "4")
	s.cmd.Stdout = os.Stderr
	s.cmd.Stderr = os.Stderr
	if err := s.cmd.Start(); err != nil {
		s.t.Fatalf("start capdb-server: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", s.addr, 200*time.Millisecond); err == nil {
			c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatalf("capdb-server not ready on %s", s.addr)
}

func (s *capdbServer) stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
		s.cmd = nil
	}
}

func (s *capdbServer) restart() {
	s.stop()
	s.start()
}

func (s *capdbServer) dsn() string {
	return fmt.Sprintf("capdb://token@%s/capper.db?token=%s&insecure=1&connect_timeout=2", s.addr, s.token)
}

func (s *capdbServer) useEnv() {
	s.t.Setenv("CAPPER_DB_DRIVER", "capdb")
	s.t.Setenv("CAPPER_DB_DSN", s.dsn())
	s.t.Setenv("CAPPER_DB_MAX_OPEN_CONNS", "2")
}

// TestStoreOpenAgainstCapDB proves the real control-plane store boots against a
// live capdb-server: store.Open runs every sub-store's schema migration over the
// network, and a round-trip through the shared *sql.DB works.
func TestStoreOpenAgainstCapDB(t *testing.T) {
	srv := newCapdbServer(t)
	srv.useEnv()

	st, err := store.Open(store.NewPaths(t.TempDir()))
	if err != nil {
		t.Fatalf("store.Open against capdb: %v", err)
	}
	defer st.DB.Close()

	if _, err := st.DB.Exec(`CREATE TABLE itest(id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := st.DB.Exec(`INSERT INTO itest(id,v) VALUES(1,'hello')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var v string
	if err := st.DB.QueryRow(`SELECT v FROM itest WHERE id=1`).Scan(&v); err != nil {
		t.Fatalf("select: %v", err)
	}
	if v != "hello" {
		t.Fatalf("round-trip mismatch: %q", v)
	}
}

// TestSelfHealAfterRestart proves R1: after capdb-server restarts, the pool
// discards the now-dead connections and queries recover quickly (vs. the old
// behavior where poisoned conns failed until ConnMaxLifetime, ~5 min).
func TestSelfHealAfterRestart(t *testing.T) {
	srv := newCapdbServer(t)
	srv.useEnv()

	st, err := store.Open(store.NewPaths(t.TempDir()))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.DB.Close()

	if _, err := st.DB.Exec(`CREATE TABLE heal(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO heal VALUES(1)`); err != nil {
		t.Fatal(err)
	}
	// Warm the pool with a couple of idle connections.
	for i := 0; i < 3; i++ {
		var n int
		_ = st.DB.QueryRow(`SELECT COUNT(*) FROM heal`).Scan(&n)
	}

	srv.restart() // data persists in db-root; old pooled conns are now dead

	// Recovery must happen within seconds, not minutes. Retry to flush the
	// poisoned (now self-evicting) connections.
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var n int
		if lastErr = st.DB.QueryRow(`SELECT COUNT(*) FROM heal`).Scan(&n); lastErr == nil {
			if n != 1 {
				t.Fatalf("after restart count = %d, want 1", n)
			}
			return // recovered
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("did not recover after restart within deadline: %v", lastErr)
}

// TestConnectFailsFast proves R2: pointing at an unreachable server fails in
// bounded time, not after a multi-minute kernel connect timeout.
func TestConnectFailsFast(t *testing.T) {
	// Grab a port and immediately release it so nothing is listening.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	t.Setenv("CAPPER_DB_DRIVER", "capdb")
	t.Setenv("CAPPER_DB_DSN", fmt.Sprintf("capdb://token@%s/capper.db?token=x&insecure=1&connect_timeout=2", addr))
	t.Setenv("CAPPER_DB_STARTUP_RETRIES", "1") // fail after one bounded attempt

	start := time.Now()
	_, err = store.Open(store.NewPaths(t.TempDir()))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected store.Open to fail against an unreachable server")
	}
	if elapsed > 20*time.Second {
		t.Fatalf("store.Open hung for %v (connect timeout not enforced)", elapsed)
	}
}
