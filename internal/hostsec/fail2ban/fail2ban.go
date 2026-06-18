// Package fail2ban is the exclusive worker that drives the host's fail2ban via
// fail2ban-client. All access is serialized through a single hostsec.Runner.
package fail2ban

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"capper/internal/hostsec"
)

// defaultAllowlistPath is the Capper-owned fail2ban drop-in that holds the
// ignoreip allowlist. Capper only ever writes this file; operator-authored jails
// under jail.d are never touched.
const defaultAllowlistPath = "/etc/fail2ban/jail.d/capper-allowlist.local"

// baseIgnore is always kept in the allowlist so the host never bans loopback.
var baseIgnore = []string{"127.0.0.1/8", "::1"}

// Worker owns exclusive access to fail2ban-client.
type Worker struct {
	runner        *hostsec.Runner
	allowlistPath string
}

// New returns a fail2ban Worker.
func New() *Worker {
	return &Worker{runner: hostsec.NewRunner("fail2ban-client"), allowlistPath: defaultAllowlistPath}
}

// NewWithRunner is used in tests to inject a fake runner.
func NewWithRunner(r *hostsec.Runner) *Worker {
	return &Worker{runner: r, allowlistPath: defaultAllowlistPath}
}

// SetAllowlistPath overrides the drop-in path (used in tests).
func (w *Worker) SetAllowlistPath(p string) { w.allowlistPath = p }

// Available reports whether fail2ban-client is installed.
func (w *Worker) Available() bool { return w.runner.Available() }

// Jail summarizes one fail2ban jail.
type Jail struct {
	Name           string   `json:"name"`
	CurrentlyBanned int     `json:"currentlyBanned"`
	TotalBanned    int      `json:"totalBanned"`
	CurrentlyFailed int     `json:"currentlyFailed"`
	BannedIPs      []string `json:"bannedIps"`
}

// Status is the overall fail2ban status.
type Status struct {
	Available bool   `json:"available"`
	Jails     []Jail `json:"jails"`
}

// Status returns the overall status and per-jail detail.
func (w *Worker) Status(ctx context.Context) (Status, error) {
	if !w.Available() {
		return Status{Available: false}, nil
	}
	out, err := w.runner.Run(ctx, "status")
	if err != nil {
		return Status{Available: true}, fmt.Errorf("fail2ban: status: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	names := parseJailList(string(out))
	st := Status{Available: true}
	for _, name := range names {
		j, jerr := w.JailStatus(ctx, name)
		if jerr != nil {
			j = Jail{Name: name}
		}
		st.Jails = append(st.Jails, j)
	}
	return st, nil
}

// JailStatus returns detail for a single jail.
func (w *Worker) JailStatus(ctx context.Context, jail string) (Jail, error) {
	out, err := w.runner.Run(ctx, "status", jail)
	if err != nil {
		return Jail{Name: jail}, fmt.Errorf("fail2ban: status %s: %w (%s)", jail, err, strings.TrimSpace(string(out)))
	}
	return parseJailStatus(jail, string(out)), nil
}

// Ban bans an IP in a jail.
func (w *Worker) Ban(ctx context.Context, jail, ip string) error {
	if jail == "" || ip == "" {
		return fmt.Errorf("fail2ban: jail and ip are required")
	}
	out, err := w.runner.Run(ctx, "set", jail, "banip", ip)
	if err != nil {
		return fmt.Errorf("fail2ban: ban %s in %s: %w (%s)", ip, jail, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Unban removes an IP ban from a jail.
func (w *Worker) Unban(ctx context.Context, jail, ip string) error {
	if jail == "" || ip == "" {
		return fmt.Errorf("fail2ban: jail and ip are required")
	}
	out, err := w.runner.Run(ctx, "set", jail, "unbanip", ip)
	if err != nil {
		return fmt.Errorf("fail2ban: unban %s in %s: %w (%s)", ip, jail, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GetAllowlist returns the admin-managed ignoreip entries (excluding the always-on
// loopback bases), read from the Capper drop-in.
func (w *Worker) GetAllowlist() ([]string, error) {
	data, err := os.ReadFile(w.allowlistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "ignoreip") {
			continue
		}
		if idx := strings.Index(line, "="); idx != -1 {
			for _, ip := range strings.Fields(line[idx+1:]) {
				if !contains(baseIgnore, ip) {
					out = append(out, ip)
				}
			}
		}
	}
	return out, nil
}

// SetAllowlist writes the ignoreip allowlist drop-in (always including loopback)
// and reloads fail2ban. Capper owns this file exclusively.
func (w *Worker) SetAllowlist(ctx context.Context, ips []string) error {
	all := append(append([]string{}, baseIgnore...), ips...)
	content := "# Managed by Capper — do not edit.\n[DEFAULT]\nignoreip = " + strings.Join(all, " ") + "\n"
	if err := os.MkdirAll(filepath.Dir(w.allowlistPath), 0o755); err != nil {
		return fmt.Errorf("fail2ban: allowlist dir: %w", err)
	}
	if err := os.WriteFile(w.allowlistPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("fail2ban: write allowlist: %w", err)
	}
	if out, err := w.runner.Run(ctx, "reload"); err != nil {
		return fmt.Errorf("fail2ban: reload: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureBans re-applies a set of (jail, ip) bans, skipping ones already in
// effect. It is idempotent and used by the persistent-blocklist reconciler.
// Returns the number of bans (re)applied.
func (w *Worker) EnsureBans(ctx context.Context, want map[string][]string) (int, error) {
	if !w.Available() {
		return 0, nil
	}
	applied := 0
	for jail, ips := range want {
		current, err := w.JailStatus(ctx, jail)
		active := map[string]bool{}
		if err == nil {
			for _, ip := range current.BannedIPs {
				active[ip] = true
			}
		}
		for _, ip := range ips {
			if active[ip] {
				continue
			}
			if err := w.Ban(ctx, jail, ip); err == nil {
				applied++
			}
		}
	}
	return applied, nil
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// parseJailList extracts jail names from `fail2ban-client status` output.
func parseJailList(s string) []string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(strings.ToLower(line), "jail list:")
		if idx == -1 {
			continue
		}
		rest := line[idx+len("jail list:"):]
		var out []string
		for _, part := range strings.Split(rest, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// parseJailStatus extracts counts and banned IPs from `status <jail>` output.
func parseJailStatus(jail, s string) Jail {
	j := Jail{Name: jail}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "currently failed:"):
			j.CurrentlyFailed = lastInt(line)
		case strings.Contains(low, "currently banned:"):
			j.CurrentlyBanned = lastInt(line)
		case strings.Contains(low, "total banned:"):
			j.TotalBanned = lastInt(line)
		case strings.Contains(low, "banned ip list:"):
			idx := strings.Index(low, "banned ip list:")
			rest := line[idx+len("banned ip list:"):]
			for _, ip := range strings.Fields(rest) {
				j.BannedIPs = append(j.BannedIPs, ip)
			}
		}
	}
	return j
}

// lastInt returns the trailing integer on a line, or 0.
func lastInt(line string) int {
	fields := strings.Fields(line)
	for i := len(fields) - 1; i >= 0; i-- {
		n := 0
		ok := true
		for _, c := range fields[i] {
			if c < '0' || c > '9' {
				ok = false
				break
			}
			n = n*10 + int(c-'0')
		}
		if ok && fields[i] != "" {
			return n
		}
	}
	return 0
}
