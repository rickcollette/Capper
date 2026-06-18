// Package ufw is the exclusive worker that drives the host's UFW firewall via
// the ufw CLI. All access is serialized through a single hostsec.Runner.
package ufw

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"capper/internal/hostsec"
)

// Worker owns exclusive access to the ufw CLI.
type Worker struct {
	runner *hostsec.Runner
}

// New returns a ufw Worker.
func New() *Worker { return &Worker{runner: hostsec.NewRunner("ufw")} }

// NewWithRunner is used in tests to inject a fake runner.
func NewWithRunner(r *hostsec.Runner) *Worker { return &Worker{runner: r} }

// Available reports whether ufw is installed.
func (w *Worker) Available() bool { return w.runner.Available() }

// Rule is a single numbered UFW rule.
type Rule struct {
	Num    int    `json:"num"`
	To     string `json:"to"`
	Action string `json:"action"` // ALLOW, DENY, REJECT, LIMIT
	From   string `json:"from"`
	Raw    string `json:"raw"`
}

// Status is the overall UFW status.
type Status struct {
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
	Rules     []Rule `json:"rules"`
}

// Status returns whether UFW is active and its numbered rules.
func (w *Worker) Status(ctx context.Context) (Status, error) {
	if !w.Available() {
		return Status{Available: false}, nil
	}
	out, err := w.runner.Run(ctx, "status", "numbered")
	if err != nil {
		return Status{Available: true}, fmt.Errorf("ufw: status: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	st := Status{Available: true}
	st.Enabled = strings.Contains(string(out), "Status: active")
	st.Rules = parseRules(string(out))
	return st, nil
}

// AllowOptions / DenyOptions are expressed via AddRule.
type AddRuleOptions struct {
	Action string // "allow" | "deny" | "reject" | "limit"
	Port   string // e.g. "22", "80/tcp"
	Proto  string // optional: "tcp" | "udp"
	From   string // optional source, e.g. "203.0.113.0/24"
	Comment string
}

// AddRule adds a rule. Capper tags its rules with a comment so they can be told
// apart from operator-authored rules.
func (w *Worker) AddRule(ctx context.Context, o AddRuleOptions) error {
	action := strings.ToLower(o.Action)
	switch action {
	case "allow", "deny", "reject", "limit":
	default:
		return fmt.Errorf("ufw: invalid action %q", o.Action)
	}
	if o.Port == "" && o.From == "" {
		return fmt.Errorf("ufw: a port or source is required")
	}
	args := []string{action}
	if o.From != "" {
		args = append(args, "from", o.From)
		if o.Port != "" {
			args = append(args, "to", "any", "port", o.Port)
		}
	} else {
		port := o.Port
		if o.Proto != "" {
			port = o.Port + "/" + o.Proto
		}
		args = append(args, port)
	}
	if o.Comment != "" {
		args = append(args, "comment", o.Comment)
	}
	out, err := w.runner.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("ufw: add rule: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteRule deletes the rule at the given 1-based number.
func (w *Worker) DeleteRule(ctx context.Context, num int) error {
	if num < 1 {
		return fmt.Errorf("ufw: rule number must be >= 1")
	}
	// `ufw --force delete N` avoids the interactive confirmation prompt.
	out, err := w.runner.Run(ctx, "--force", "delete", strconv.Itoa(num))
	if err != nil {
		return fmt.Errorf("ufw: delete rule %d: %w (%s)", num, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetEnabled enables or disables the firewall. Enabling uses --force to skip the
// "this may disrupt existing ssh connections" prompt; callers must ensure an
// allow rule for SSH/the control plane first.
func (w *Worker) SetEnabled(ctx context.Context, enabled bool) error {
	verb := "disable"
	args := []string{"disable"}
	if enabled {
		verb = "enable"
		args = []string{"--force", "enable"}
	}
	out, err := w.runner.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("ufw: %s: %w (%s)", verb, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Defaults holds UFW's default policies.
type Defaults struct {
	Incoming string `json:"incoming"` // allow | deny | reject
	Outgoing string `json:"outgoing"`
	Routed   string `json:"routed"`
}

// GetDefaults reads the default policies from `ufw status verbose`.
func (w *Worker) GetDefaults(ctx context.Context) (Defaults, error) {
	out, err := w.runner.Run(ctx, "status", "verbose")
	if err != nil {
		return Defaults{}, fmt.Errorf("ufw: status verbose: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return parseDefaults(string(out)), nil
}

// SetDefault sets a default policy for a direction (incoming|outgoing|routed).
func (w *Worker) SetDefault(ctx context.Context, direction, policy string) error {
	direction = strings.ToLower(direction)
	policy = strings.ToLower(policy)
	switch direction {
	case "incoming", "outgoing", "routed":
	default:
		return fmt.Errorf("ufw: invalid direction %q", direction)
	}
	switch policy {
	case "allow", "deny", "reject":
	default:
		return fmt.Errorf("ufw: invalid policy %q", policy)
	}
	out, err := w.runner.Run(ctx, "default", policy, direction)
	if err != nil {
		return fmt.Errorf("ufw: default %s %s: %w (%s)", policy, direction, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// parseDefaults extracts the default-policy line from `ufw status verbose`, e.g.
// "Default: deny (incoming), allow (outgoing), disabled (routed)".
func parseDefaults(s string) Defaults {
	var d Defaults
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Default:") {
			continue
		}
		for _, part := range strings.Split(strings.TrimPrefix(line, "Default:"), ",") {
			fields := strings.Fields(strings.TrimSpace(part))
			if len(fields) < 2 {
				continue
			}
			policy := fields[0]
			dir := strings.Trim(fields[1], "()")
			switch dir {
			case "incoming":
				d.Incoming = policy
			case "outgoing":
				d.Outgoing = policy
			case "routed":
				d.Routed = policy
			}
		}
	}
	return d
}

// parseRules parses `ufw status numbered` output into structured rules.
func parseRules(s string) []Rule {
	var rules []Rule
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		close := strings.Index(line, "]")
		if close == -1 {
			continue
		}
		numStr := strings.TrimSpace(line[1:close])
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		rest := strings.TrimSpace(line[close+1:])
		r := Rule{Num: num, Raw: rest}
		// Format: "<to>  <ACTION>  <from>" with action being ALLOW/DENY/etc.
		for _, act := range []string{"ALLOW", "DENY", "REJECT", "LIMIT"} {
			if idx := strings.Index(rest, act); idx != -1 {
				r.To = strings.TrimSpace(rest[:idx])
				r.Action = act
				tail := strings.TrimSpace(rest[idx+len(act):])
				tail = strings.TrimPrefix(tail, "IN")
				r.From = strings.TrimSpace(tail)
				break
			}
		}
		rules = append(rules, r)
	}
	return rules
}
