package cli

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ---- systemd unit templates ------------------------------------------------

const capperControlUnit = `[Unit]
Description=Capper Control Plane
After=network.target

[Service]
Environment=HOME=/root
ExecStart=/usr/local/bin/capper api start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`

const capperAgentUnit = `[Unit]
Description=Capper Node Agent
After=capper-control.service

[Service]
Environment=HOME=/root
ExecStart=/usr/local/bin/capper-agent --config /etc/capper/agent.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`

// capdb-server unit, rendered with listen/auth/db-root/pool/tls args.
const capdbServerUnitTmpl = `[Unit]
Description=CapDB SQL server
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/capdb-server --listen %s --auth-file %s --db-root %s --pool-max %d%s
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`

// capper-control unit when the CapDB backend is selected: pulls DB settings from
// an env file and waits for the CapDB service.
const capperControlCapDBUnit = `[Unit]
Description=Capper Control Plane
After=network.target capdb-server.service
Requires=capdb-server.service

[Service]
Environment=HOME=/root
EnvironmentFile=/etc/capper/capdb.env
ExecStart=/usr/local/bin/capper api start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`

// ---- aio root command ------------------------------------------------------

func aioCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aio",
		Short: "All-in-one single-node Capper management",
	}
	cmd.AddCommand(
		aioInitCmd(),
		aioUpCmd(),
		aioDownCmd(),
		aioStatusCmd(),
		aioResetCmd(),
		aioLogsCmd(),
		aioDoctorCmd(),
		aioUpgradeCmd(opts),
		aioCheckUpdateCmd(opts),
	)
	return cmd
}

// ---- init ------------------------------------------------------------------

func aioInitCmd() *cobra.Command {
	var name, storageRoot, backend string
	var insecure bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise AIO node directories and config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = "devbox"
			}
			if storageRoot == "" {
				storageRoot = "/var/lib/capper"
			}
			if backend == "" {
				backend = "sqlite"
			}
			if backend != "sqlite" && backend != "capdb" {
				return fmt.Errorf("--backend must be sqlite or capdb, got %q", backend)
			}

			for _, sub := range []string{
				"control-plane", "images", "instances",
				"networks", "storage/volumes", "storage/buckets",
				"s3/data", "registry", "logs",
			} {
				if err := os.MkdirAll(filepath.Join(storageRoot, sub), 0o750); err != nil {
					return err
				}
			}

			agentYAML := fmt.Sprintf(`node:
  name: %s
  realm: local
  region: local
  zone: local-a
  roles:
    - all-in-one
    - control-plane
    - compute
    - shared-disk
    - s3
    - network
    - ingress
  failureDomain: local-0
controlPlane:
  url: http://localhost:8080
  heartbeatInterval: 10s
  tlsVerify: false
`, name)
			if err := os.MkdirAll("/etc/capper", 0o755); err != nil {
				return err
			}
			if err := os.WriteFile("/etc/capper/agent.yaml", []byte(agentYAML), 0o644); err != nil {
				return err
			}

			aioYAML := fmt.Sprintf("name: %s\nstorage: %s\nmode: all-in-one\n", name, storageRoot)
			if err := os.WriteFile("/etc/capper/aio.yaml", []byte(aioYAML), 0o644); err != nil {
				return err
			}

			// CapDB backend: provision the server's data-root, credential file,
			// TLS material, and env file consumed by the control plane, and stage
			// its systemd unit. TLS is the default; --insecure is dev-only.
			if backend == "capdb" {
				if err := provisionCapDB(storageRoot, insecure); err != nil {
					return fmt.Errorf("provision capdb: %w", err)
				}
			}

			if _, err := exec.LookPath("systemctl"); err == nil {
				if backend == "capdb" {
					writeSystemdUnit("/etc/systemd/system/capdb-server.service", renderCapDBUnit(storageRoot, insecure))
					writeSystemdUnit("/etc/systemd/system/capper-control.service", capperControlCapDBUnit)
				} else {
					writeSystemdUnit("/etc/systemd/system/capper-control.service", capperControlUnit)
				}
				writeSystemdUnit("/etc/systemd/system/capper-agent.service", capperAgentUnit)
				_ = exec.Command("systemctl", "daemon-reload").Run()
			}

			fmt.Printf("AIO node %q initialised (backend: %s). Run: capper aio up\n", name, backend)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "node name slug (default: devbox)")
	cmd.Flags().StringVar(&storageRoot, "storage", "", "storage root path (default: /var/lib/capper)")
	cmd.Flags().StringVar(&backend, "backend", "", "database backend: sqlite (default) or capdb")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "capdb backend: disable TLS (dev only; default is TLS)")
	return cmd
}

// Co-located CapDB defaults for aio. The server binds loopback; with TLS the
// client connects via the "localhost" DNS name so certificate hostname
// verification works (an IP SAN would otherwise be needed).
const (
	capdbListen   = "127.0.0.1:5432"
	capdbTLSHost  = "localhost:5432"
	capdbDBName   = "capper.db"
	capdbAuthFile = "/etc/capper/capdb.auth"
	capdbCertFile = "/etc/capper/tls/capdb.crt"
	capdbKeyFile  = "/etc/capper/tls/capdb.key"
)

// provisionCapDB creates the db-root, a random token credential file, optional
// TLS material, and the env file consumed by the control plane (and aio up). The
// token is written to a 0600 file and referenced via CAPPER_DB_TOKEN_FILE — it is
// NOT placed in the persisted DSN/env (the control plane injects it at connect
// time). TLS is the default; pass insecure=true (dev only) for plaintext loopback.
func provisionCapDB(storageRoot string, insecure bool) error {
	dbRoot := filepath.Join(storageRoot, "capdb")
	if err := os.MkdirAll(dbRoot, 0o700); err != nil {
		return err
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		return err
	}
	if err := os.WriteFile(capdbAuthFile, []byte(hex.EncodeToString(tok)+"\n"), 0o600); err != nil {
		return err
	}

	var dsn string
	if insecure {
		dsn = fmt.Sprintf("capdb://token@%s/%s?insecure=1", capdbListen, capdbDBName)
	} else {
		if err := os.MkdirAll(filepath.Dir(capdbCertFile), 0o755); err != nil {
			return err
		}
		if err := generateSelfSignedCert("localhost", capdbCertFile, capdbKeyFile); err != nil {
			return fmt.Errorf("generate capdb TLS cert: %w", err)
		}
		dsn = fmt.Sprintf("capdb://token@%s/%s?ca=%s", capdbTLSHost, capdbDBName, capdbCertFile)
	}

	// Token is referenced by file, not embedded in the DSN/env.
	env := "CAPPER_DB_DRIVER=capdb\n" +
		"CAPPER_DB_DSN=" + dsn + "\n" +
		"CAPPER_DB_TOKEN_FILE=" + capdbAuthFile + "\n" +
		"CAPPER_DB_MAX_OPEN_CONNS=8\n" +
		"CAPPER_CAPDB_DB_ROOT=" + dbRoot + "\n"
	return os.WriteFile("/etc/capper/capdb.env", []byte(env), 0o600)
}

func renderCapDBUnit(storageRoot string, insecure bool) string {
	dbRoot := filepath.Join(storageRoot, "capdb")
	tls := fmt.Sprintf(" --cert %s --key %s", capdbCertFile, capdbKeyFile)
	if insecure {
		tls = " --insecure"
	}
	return fmt.Sprintf(capdbServerUnitTmpl, capdbListen, capdbAuthFile, dbRoot, 8, tls)
}

// generateSelfSignedCert writes a P-256 self-signed certificate + key for the
// given host (DNS SAN) to certPath/keyPath. Used by `aio init --backend capdb`
// so the co-located control-plane↔DB channel is TLS by default.
func generateSelfSignedCert(host, certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(keyPath, keyPEM, 0o600)
}

// ---- up --------------------------------------------------------------------

func aioUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start AIO services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("systemctl"); err == nil {
				svcs := []string{"capper-control", "capper-agent"}
				// Start the CapDB service first when it was provisioned (its unit
				// exists); capper-control Requires it.
				if unitExists("capdb-server.service") {
					svcs = append([]string{"capdb-server"}, svcs...)
				}
				for _, svc := range svcs {
					out, err := exec.Command("systemctl", "start", svc).CombinedOutput()
					if err != nil {
						return fmt.Errorf("start %s: %w\n%s", svc, err, out)
					}
				}
				return waitForHealth("http://localhost:8080/api/v1/health", 30*time.Second)
			}
			// Direct process management.
			// Optionally co-locate a capdb-server when the CapDB backend is
			// selected (no-op for the default modernc backend).
			capdbPid, err := maybeStartCapDBServer()
			if err != nil {
				return err
			}
			ctrlPid, err := spawnBackground("capper", "api", "start")
			if err != nil {
				return fmt.Errorf("start capper control: %w", err)
			}
			if err := waitForHealth("http://localhost:8080/api/v1/health", 30*time.Second); err != nil {
				return err
			}
			agentPid, err := spawnBackground("capper-agent", "--config", "/etc/capper/agent.yaml")
			if err != nil {
				return fmt.Errorf("start capper-agent: %w", err)
			}
			pids := []int{ctrlPid, agentPid}
			if capdbPid > 0 {
				pids = append(pids, capdbPid)
			}
			savePIDs(pids...)
			fmt.Println("AIO node is up.")
			return nil
		},
	}
}

// ---- down ------------------------------------------------------------------

func aioDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop AIO services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("systemctl"); err == nil {
				svcs := []string{"capper-agent", "capper-control"}
				if unitExists("capdb-server.service") {
					svcs = append(svcs, "capdb-server") // stop DB last
				}
				out, err := exec.Command("systemctl", append([]string{"stop"}, svcs...)...).CombinedOutput()
				if err != nil {
					return fmt.Errorf("stop services: %w\n%s", err, out)
				}
				fmt.Println("AIO services stopped.")
				return nil
			}
			return stopPIDs("/run/capper-aio.pid", 10*time.Second)
		},
	}
}

// unitExists reports whether a systemd unit file has been written for a service.
func unitExists(unit string) bool {
	_, err := os.Stat(filepath.Join("/etc/systemd/system", unit))
	return err == nil
}

// ---- status ----------------------------------------------------------------

func aioStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show AIO node status",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiOK := false
			resp, err := http.Get("http://localhost:8080/api/v1/health")
			if err == nil {
				resp.Body.Close()
				apiOK = resp.StatusCode == http.StatusOK
			}
			if apiOK {
				fmt.Println("control plane: UP")
			} else {
				fmt.Println("control plane: OFFLINE")
			}

			// CapDB service (only when this node uses the CapDB backend).
			if unitExists("capdb-server.service") || os.Getenv("CAPPER_DB_DRIVER") == "capdb" {
				addr := capdbListen
				if dsn := os.Getenv("CAPPER_DB_DSN"); dsn != "" {
					if u, e := url.Parse(dsn); e == nil && u.Host != "" {
						addr = u.Host
					}
				}
				if c, e := net.DialTimeout("tcp", addr, time.Second); e == nil {
					c.Close()
					fmt.Printf("capdb: UP (%s)\n", addr)
				} else {
					fmt.Printf("capdb: OFFLINE (%s)\n", addr)
				}
			}

			// Query local agent socket.
			data, sockErr := queryAgentSocket("/status")
			if sockErr != nil {
				fmt.Println("agent: OFFLINE")
				return nil
			}
			fmt.Println("agent: UP")
			fmt.Println(string(data))
			return nil
		},
	}
}

// ---- reset -----------------------------------------------------------------

func aioResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Stop AIO, remove state, and clear node identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = aioDownCmd().RunE(cmd, args)

			storageRoot := aioStorageRoot()
			for _, sub := range []string{
				"control-plane", "images", "instances",
				"networks", "storage", "s3", "registry", "logs",
			} {
				_ = os.RemoveAll(filepath.Join(storageRoot, sub))
			}
			_ = os.Remove("/etc/capper/node-id")
			_ = os.Remove("/etc/capper/agent-token")
			fmt.Println("AIO node reset. Run: capper aio up")
			return nil
		},
	}
}

// ---- logs ------------------------------------------------------------------

func aioLogsCmd() *cobra.Command {
	var service string
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream AIO service logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("systemctl"); err == nil {
				jArgs := []string{"-n", strconv.Itoa(tail), "-f"}
				if service == "control" || service == "" {
					jArgs = append(jArgs, "-u", "capper-control")
				}
				if service == "agent" || service == "" {
					jArgs = append(jArgs, "-u", "capper-agent")
				}
				c := exec.Command("journalctl", jArgs...)
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				return c.Run()
			}
			storageRoot := aioStorageRoot()
			var logFiles []string
			if service == "control" || service == "" {
				logFiles = append(logFiles, filepath.Join(storageRoot, "logs", "control.log"))
			}
			if service == "agent" || service == "" {
				logFiles = append(logFiles, filepath.Join(storageRoot, "logs", "agent.log"))
			}
			tailArgs := append([]string{"-n", strconv.Itoa(tail), "-f"}, logFiles...)
			c := exec.Command("tail", tailArgs...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
	cmd.Flags().StringVar(&service, "service", "", "service to stream: control, agent (default: both)")
	cmd.Flags().IntVar(&tail, "tail", 100, "number of recent lines to show")
	return cmd
}

// ---- doctor ----------------------------------------------------------------

func aioDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run AIO pre-flight checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			storageRoot := aioStorageRoot()
			type healthCheck struct {
				name string
				fn   func() (string, error)
			}
			checks := []healthCheck{
				{"cgroup v2 available", checkCgroupV2},
				{"disk space (>= 10 GiB free)", func() (string, error) { return checkDiskFree(storageRoot, 10<<30) }},
				{"port 8080 available", func() (string, error) { return checkPortFree(8080) }},
				{"port 8333 available (S3)", func() (string, error) { return checkPortFree(8333) }},
				{"port 8443 available (HTTPS)", func() (string, error) { return checkPortFree(8443) }},
				{"capper binary in PATH", func() (string, error) { return exec.LookPath("capper") }},
				{"capper-agent binary in PATH", func() (string, error) { return exec.LookPath("capper-agent") }},
			}
			allPass := true
			for _, c := range checks {
				detail, err := c.fn()
				if err != nil {
					fmt.Printf("FAIL  %s: %v\n", c.name, err)
					allPass = false
				} else {
					fmt.Printf("PASS  %s%s\n", c.name, fmtDetail(detail))
				}
			}
			if !allPass {
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}
}

// ---- helpers ---------------------------------------------------------------

func writeSystemdUnit(path, content string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(content), 0o644)
}

func waitForHealth(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timed out waiting for %s", url)
}

// maybeStartCapDBServer launches a co-located capdb-server when the control
// plane is configured to use the CapDB backend (CAPPER_DB_DRIVER=capdb), wiring
// it to a local db-root and an auth file derived from the DSN token. For the
// default modernc backend it is a no-op (returns pid 0). The server binary is
// located via CAPPER_CAPDB_SERVER (path) or $PATH. TLS is used when
// CAPPER_CAPDB_CERT/KEY are set; otherwise (or with insecure=1 in the DSN) it
// runs in insecure mode for local single-node use.
func maybeStartCapDBServer() (int, error) {
	if os.Getenv("CAPPER_DB_DRIVER") != "capdb" {
		return 0, nil
	}
	dsn := os.Getenv("CAPPER_DB_DSN")
	if dsn == "" {
		return 0, fmt.Errorf("CAPPER_DB_DRIVER=capdb requires CAPPER_DB_DSN")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "capdb" {
		return 0, fmt.Errorf("invalid CAPPER_DB_DSN %q: %v", dsn, err)
	}

	bin := os.Getenv("CAPPER_CAPDB_SERVER")
	if bin == "" {
		bin = "capdb-server"
	}
	if _, err := exec.LookPath(bin); err != nil {
		if _, statErr := os.Stat(bin); statErr != nil {
			return 0, fmt.Errorf("capdb-server not found (set CAPPER_CAPDB_SERVER or add to PATH): %w", err)
		}
	}

	listen := u.Host
	if listen == "" {
		listen = "127.0.0.1:5432"
	}
	q := u.Query()
	token := q.Get("token")
	if token == "" {
		token = "local"
	}

	dbRoot := os.Getenv("CAPPER_CAPDB_DB_ROOT")
	if dbRoot == "" {
		home, _ := os.UserHomeDir()
		dbRoot = filepath.Join(home, ".capper", "capdb")
	}
	if err := os.MkdirAll(dbRoot, 0o700); err != nil {
		return 0, fmt.Errorf("capdb db-root: %w", err)
	}
	authFile := filepath.Join(dbRoot, "auth.txt")
	if err := os.WriteFile(authFile, []byte(token+"\n"), 0o600); err != nil {
		return 0, fmt.Errorf("capdb auth file: %w", err)
	}

	// Size the server pool to the client's MaxOpenConns so client connections
	// never block on a server-side pool acquire (see internal/store/open.go).
	poolMax := 8
	if v, err := strconv.Atoi(os.Getenv("CAPPER_DB_MAX_OPEN_CONNS")); err == nil && v > 0 {
		poolMax = v
	}
	args := []string{
		"--listen", listen,
		"--auth-file", authFile,
		"--db-root", dbRoot,
		"--pool-max", strconv.Itoa(poolMax),
	}
	cert, key := os.Getenv("CAPPER_CAPDB_CERT"), os.Getenv("CAPPER_CAPDB_KEY")
	if q.Get("insecure") == "1" || cert == "" || key == "" {
		args = append(args, "--insecure")
	} else {
		args = append(args, "--cert", cert, "--key", key)
	}

	pid, err := spawnBackground(bin, args...)
	if err != nil {
		return 0, fmt.Errorf("start capdb-server: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if c, derr := net.DialTimeout("tcp", listen, 300*time.Millisecond); derr == nil {
			c.Close()
			fmt.Printf("co-located capdb-server up on %s (db-root %s)\n", listen, dbRoot)
			return pid, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return pid, fmt.Errorf("capdb-server did not become ready on %s", listen)
}

func spawnBackground(name string, args ...string) (int, error) {
	c := exec.Command(name, args...)
	c.Stdout = nil
	c.Stderr = nil
	if err := c.Start(); err != nil {
		return 0, err
	}
	return c.Process.Pid, nil
}

func savePIDs(pids ...int) {
	parts := make([]string, len(pids))
	for i, p := range pids {
		parts[i] = strconv.Itoa(p)
	}
	_ = os.WriteFile("/run/capper-aio.pid", []byte(strings.Join(parts, "\n")), 0o644)
}

func stopPIDs(pidFile string, wait time.Duration) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // already stopped
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		_ = proc.Signal(os.Interrupt)
	}
	time.Sleep(wait)
	_ = os.Remove(pidFile)
	fmt.Println("AIO services stopped.")
	return nil
}

func queryAgentSocket(path string) ([]byte, error) {
	c := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/capper-agent.sock")
			},
		},
	}
	resp, err := c.Get("http://localhost" + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func aioStorageRoot() string {
	data, err := os.ReadFile("/etc/capper/aio.yaml")
	if err != nil {
		return "/var/lib/capper"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "storage: "); ok {
			return strings.TrimSpace(after)
		}
	}
	return "/var/lib/capper"
}

func checkCgroupV2() (string, error) {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return "", fmt.Errorf("cgroup v2 not mounted at /sys/fs/cgroup")
	}
	return "/sys/fs/cgroup", nil
}

func checkDiskFree(path string, minBytes int64) (string, error) {
	// Use df to avoid importing syscall/unix.
	out, err := exec.Command("df", "-B1", "--output=avail", path).Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("df: unexpected output")
	}
	avail, err := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil {
		return "", err
	}
	if avail < minBytes {
		return "", fmt.Errorf("only %d GiB free", avail>>30)
	}
	return fmt.Sprintf("(%d GiB free)", avail>>30), nil
}

func checkPortFree(port int) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return "", fmt.Errorf("port %d in use", port)
	}
	ln.Close()
	return "", nil
}

func fmtDetail(s string) string {
	if s == "" {
		return ""
	}
	return "  " + s
}

