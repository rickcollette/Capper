package posture

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scanner runs posture checks and stores findings.
type Scanner struct {
	store *Store
}

func NewScanner(s *Store) *Scanner { return &Scanner{store: s} }

// Scan runs all checks rooted at rootDir and under the given project.
// Pass rootDir="" to skip filesystem checks (host-only mode).
func (sc *Scanner) Scan(project, rootDir string) (ScanResult, error) {
	scanID := newScanID()
	now := time.Now().UTC().Format(time.RFC3339)

	var findings []Finding

	// Check 1: world-writable files (non-symlink, excluding /proc and /sys).
	if rootDir != "" {
		wwf := worldWritable(rootDir, scanID, project, now)
		findings = append(findings, wwf...)
	}

	// Check 2: SUID/SGID binaries.
	if rootDir != "" {
		suid := suidFiles(rootDir, scanID, project, now)
		findings = append(findings, suid...)
	}

	// Check 3: open listening ports on the host.
	ports := openPorts(scanID, project, now)
	findings = append(findings, ports...)

	if err := sc.store.InsertAll(findings); err != nil {
		return ScanResult{}, fmt.Errorf("posture: store findings: %w", err)
	}
	return ScanResult{
		ScanID:    scanID,
		Project:   project,
		ScannedAt: now,
		Findings:  findings,
	}, nil
}

// List returns stored findings for the project.
func (sc *Scanner) List(project string) ([]Finding, error) {
	return sc.store.ListByProject(project)
}

func worldWritable(root, scanID, project, now string) []Finding {
	var out []Finding
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := strings.TrimPrefix(path, root)
		if strings.HasPrefix(rel, "/proc") || strings.HasPrefix(rel, "/sys") {
			return fs.SkipDir
		}
		info, ierr := d.Info()
		if ierr != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if info.Mode()&0o002 != 0 {
			sev := SeverityMedium
			if info.IsDir() {
				sev = SeverityHigh
			}
			out = append(out, Finding{
				ID:        newID(),
				ScanID:    scanID,
				Project:   project,
				Check:     "world-writable",
				Severity:  sev,
				Target:    path,
				Detail:    fmt.Sprintf("mode %04o", info.Mode().Perm()),
				ScannedAt: now,
			})
		}
		return nil
	})
	return out
}

func suidFiles(root, scanID, project, now string) []Finding {
	var out []Finding
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := strings.TrimPrefix(path, root)
		if strings.HasPrefix(rel, "/proc") || strings.HasPrefix(rel, "/sys") {
			return fs.SkipDir
		}
		info, ierr := d.Info()
		if ierr != nil || !info.Mode().IsRegular() {
			return nil
		}
		mode := info.Mode()
		if mode&os.ModeSetuid != 0 || mode&os.ModeSetgid != 0 {
			out = append(out, Finding{
				ID:        newID(),
				ScanID:    scanID,
				Project:   project,
				Check:     "suid-sgid",
				Severity:  SeverityMedium,
				Target:    path,
				Detail:    fmt.Sprintf("mode %04o", mode.Perm()),
				ScannedAt: now,
			})
		}
		return nil
	})
	return out
}

func openPorts(scanID, project, now string) []Finding {
	var out []Finding
	// Read /proc/net/tcp and /proc/net/tcp6 for listening sockets.
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] { // skip header
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			// state 0A = TCP_LISTEN
			if fields[3] != "0A" {
				continue
			}
			localAddr := fields[1]
			port, perr := hexAddrPort(localAddr)
			if perr != nil {
				continue
			}
			out = append(out, Finding{
				ID:        newID(),
				ScanID:    scanID,
				Project:   project,
				Check:     "open-ports",
				Severity:  SeverityInfo,
				Target:    fmt.Sprintf(":%d", port),
				Detail:    fmt.Sprintf("listening on port %d (from %s)", port, filepath.Base(f)),
				ScannedAt: now,
			})
		}
	}
	return out
}

// hexAddrPort parses "0100007F:0050" → 80 (little-endian hex port).
func hexAddrPort(addr string) (int, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid addr")
	}
	var port uint32
	if _, err := fmt.Sscanf(parts[1], "%X", &port); err != nil {
		return 0, err
	}
	// Validate the IP portion exists (ignore result — we only care about port).
	_ = net.ParseIP(parts[0])
	return int(port), nil
}
