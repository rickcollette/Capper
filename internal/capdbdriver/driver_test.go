//go:build capdb

package capdbdriver

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// startServer boots a capdb-server in insecure (plain-TCP) mode against a temp
// db-root and returns a ready-to-use *sql.DB plus a cleanup func.
func startServer(t *testing.T) *sql.DB {
	t.Helper()

	serverBin := os.Getenv("CAPDB_SERVER")
	if serverBin == "" {
		serverBin = filepath.FromSlash("../../build/capdb/capdb-server")
	}
	if _, err := os.Stat(serverBin); err != nil {
		t.Skipf("capdb-server not built (%s); run `make capdb`", serverBin)
	}

	dir := t.TempDir()
	const token = "test-token"
	authFile := filepath.Join(dir, "auth.txt")
	if err := os.WriteFile(authFile, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Grab a free port, then let the server bind it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	cmd := exec.Command(serverBin,
		"--insecure",
		"--listen", fmt.Sprintf("127.0.0.1:%d", port),
		"--auth-file", authFile,
		"--db-root", dir,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start capdb-server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// Wait for the listener to accept.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	dsn := fmt.Sprintf("capdb://%s/test.db?token=%s&insecure=1", addr, token)
	db, err := sql.Open("capdb", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })

	// Retry the first ping while the server finishes coming up.
	for time.Now().Before(deadline) {
		if err = db.Ping(); err == nil {
			return db
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("capdb server never became ready: %v", err)
	return nil
}

func TestConformance(t *testing.T) {
	db := startServer(t)

	if _, err := db.Exec(`CREATE TABLE t(
		id    INTEGER PRIMARY KEY,
		name  TEXT,
		score REAL,
		data  BLOB,
		note  TEXT
	)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Insert with bound args of every supported type, including NULL and BLOB.
	blob := []byte{0x00, 0x01, 0xfe, 0xff}
	if _, err := db.Exec(`INSERT INTO t(id,name,score,data,note) VALUES(?,?,?,?,?)`,
		int64(1), "alice", 9.5, blob, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO t(id,name,score,data,note) VALUES(?,?,?,?,?)`,
		int64(2), "bob's \"quote\"", 3.25, []byte{}, "ok"); err != nil {
		t.Fatalf("insert2: %v", err)
	}

	// Read back and verify each type round-trips.
	var (
		name string
		nm   sql.NullString
		score float64
		data  []byte
	)
	row := db.QueryRow(`SELECT name,score,data,note FROM t WHERE id=?`, int64(1))
	if err := row.Scan(&name, &score, &data, &nm); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if name != "alice" || score != 9.5 || string(data) != string(blob) || nm.Valid {
		t.Fatalf("roundtrip mismatch: name=%q score=%v data=%x note.valid=%v", name, score, data, nm.Valid)
	}

	// Escaping: the quote-laden value must survive intact.
	var bobName string
	if err := db.QueryRow(`SELECT name FROM t WHERE id=?`, int64(2)).Scan(&bobName); err != nil {
		t.Fatalf("scan bob: %v", err)
	}
	if bobName != `bob's "quote"` {
		t.Fatalf("escaping failed: got %q", bobName)
	}

	// RowsAffected on UPDATE.
	res, err := db.Exec(`UPDATE t SET score=score+1 WHERE id IN (1,2)`)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 2 {
		t.Fatalf("RowsAffected = %d, want 2", n)
	}

	// RowsAffected = 0 for a no-match UPDATE (Capper relies on this).
	res, err = db.Exec(`UPDATE t SET score=0 WHERE id=?`, int64(999))
	if err != nil {
		t.Fatalf("update nomatch: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Fatalf("RowsAffected(no match) = %d, want 0", n)
	}

	// COUNT via aggregate.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

// TestTransactions verifies real interactive transactions: a committed tx
// persists, a rolled-back tx is fully undone, and multiple statements in one tx
// are atomic.
func TestTransactions(t *testing.T) {
	db := startServer(t)
	if _, err := db.Exec(`CREATE TABLE c(n INTEGER)`); err != nil {
		t.Fatal(err)
	}

	// Commit path persists, multiple statements in one tx.
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO c VALUES(?)`, int64(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO c VALUES(?)`, int64(2)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Rollback truly undoes the inserts.
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO c VALUES(?)`, int64(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO c VALUES(?)`, int64(4)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM c`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("after commit(2) + rollback(2): count = %d, want 2 (rollback undoes)", n)
	}

	// A read inside a transaction sees the transaction's own uncommitted writes.
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO c VALUES(?)`, int64(99)); err != nil {
		t.Fatal(err)
	}
	var inTx int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM c`).Scan(&inTx); err != nil {
		t.Fatal(err)
	}
	if inTx != 3 {
		t.Fatalf("read-your-writes in tx: count = %d, want 3", inTx)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

// TestLargeResultSet crosses the client's row-prefetch batch boundary (256) to
// exercise the multi-batch "more rows pending" path.
func TestLargeResultSet(t *testing.T) {
	db := startServer(t)
	if _, err := db.Exec(`CREATE TABLE big(n INTEGER, s TEXT)`); err != nil {
		t.Fatal(err)
	}
	const N = 600
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < N; i++ {
		if _, err := tx.Exec(`INSERT INTO big VALUES(?,?)`, int64(i), fmt.Sprintf("row-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`SELECT n, s FROM big ORDER BY n`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := 0
	for rows.Next() {
		var n int
		var s string
		if err := rows.Scan(&n, &s); err != nil {
			t.Fatal(err)
		}
		if n != got || s != fmt.Sprintf("row-%d", got) {
			t.Fatalf("row %d mismatch: n=%d s=%q", got, n, s)
		}
		got++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got != N {
		t.Fatalf("read %d rows, want %d", got, N)
	}
}

// TestQueryContextCancel verifies a cancelled context aborts a query.
func TestQueryContextCancel(t *testing.T) {
	db := startServer(t)
	if _, err := db.Exec(`CREATE TABLE q(n INTEGER)`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	if _, err := db.QueryContext(ctx, `SELECT 1`); err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO q VALUES(1)`); err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// The pool should recover with a fresh connection for subsequent work.
	if _, err := db.Exec(`INSERT INTO q VALUES(2)`); err != nil {
		t.Fatalf("post-cancel exec: %v", err)
	}
}

// TestCTEWritePersists guards the sqlIsWrite fix: a WITH...INSERT (a write
// wrapped in a CTE) must be routed to a write connection and committed, not
// classified read-only and silently rolled back on release.
func TestCTEWritePersists(t *testing.T) {
	db := startServer(t)
	if _, err := db.Exec(`CREATE TABLE w(id INTEGER, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`WITH src(id,v) AS (VALUES(1,'a'),(2,'b')) INSERT INTO w SELECT id,v FROM src`,
	); err != nil {
		t.Fatalf("CTE insert: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM w`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("CTE write not persisted: count = %d, want 2", n)
	}
}

// TestLastInsertId verifies last_insert_rowid() flows back through EXEC_RESULT.
func TestLastInsertId(t *testing.T) {
	db := startServer(t)
	if _, err := db.Exec(`CREATE TABLE r(id INTEGER PRIMARY KEY AUTOINCREMENT, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO r(v) VALUES(?)`, "x")
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	if id != 1 {
		t.Fatalf("LastInsertId = %d, want 1", id)
	}
}

func TestConcurrent(t *testing.T) {
	db := startServer(t)
	if _, err := db.Exec(`CREATE TABLE k(id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := db.Exec(`INSERT INTO k(id,v) VALUES(?,?)`, int64(i), fmt.Sprintf("v%d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent insert: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM k`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != n {
		t.Fatalf("concurrent count = %d, want %d", count, n)
	}
}
