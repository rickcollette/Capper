package firewall

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// DefaultMode is the firewall mode applied to new networks.
const DefaultMode = ModeStrict

// Manager orchestrates firewall lifecycle for virtual networks.
type Manager struct {
	store *Store
}

// NewManager creates a Manager backed by store.
func NewManager(s *Store) *Manager {
	return &Manager{store: s}
}

// Init creates a Firewall policy record for the given network.
// Idempotent: if a firewall already exists for the network it is returned unchanged.
func (m *Manager) Init(networkID, networkName, mode string) (Firewall, error) {
	existing, err := m.store.Get(networkID)
	if err == nil {
		return existing, nil // already exists
	}

	if mode == "" {
		mode = DefaultMode
	}

	fw := Firewall{
		NetworkID:            networkID,
		NetworkName:          networkName,
		Mode:                 mode,
		Backend:              "nftables",
		DefaultForwardPolicy: defaultPolicy(mode),
		DefaultIngressPolicy: defaultPolicy(mode),
		DefaultEgressPolicy:  defaultPolicy(mode),
		AllowDNS:             true,
		AllowEstablished:     true,
		NATEnabled:           false,
		Status:               StatusPending,
	}

	if err := m.store.Insert(fw); err != nil {
		return Firewall{}, err
	}
	return fw, nil
}

// AddRule appends a new Rule to the network's policy.
func (m *Manager) AddRule(networkID string, spec RuleSpec) (Rule, error) {
	if spec.Action == "" {
		spec.Action = ActionDeny
	}
	if spec.Direction == "" {
		spec.Direction = DirectionForward
	}
	if spec.Protocol == "" {
		spec.Protocol = "any"
	}
	if spec.From.Type == "" {
		spec.From.Type = EndpointAny
	}
	if spec.To.Type == "" {
		spec.To.Type = EndpointAny
	}
	if spec.Priority == 0 {
		spec.Priority = defaultPriority(spec.Action)
	}

	r := Rule{
		ID:          newRuleID(),
		NetworkID:   networkID,
		Priority:    spec.Priority,
		Enabled:     true,
		Action:      spec.Action,
		Direction:   spec.Direction,
		From:        spec.From,
		To:          spec.To,
		Protocol:    spec.Protocol,
		Ports:       spec.Ports,
		Description: spec.Description,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if err := m.store.InsertRule(r); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// DeleteRule removes a rule from the network.
func (m *Manager) DeleteRule(networkID, ruleID string) error {
	r, err := m.store.GetRule(ruleID)
	if err != nil {
		return err
	}
	if r.NetworkID != networkID {
		return fmt.Errorf("firewall: rule %q does not belong to network %q", ruleID, networkID)
	}
	return m.store.DeleteRule(ruleID)
}

// EnableRule re-enables a previously disabled rule.
func (m *Manager) EnableRule(networkID, ruleID string) error {
	return m.setRuleEnabled(networkID, ruleID, true)
}

// DisableRule disables a rule without deleting it.
func (m *Manager) DisableRule(networkID, ruleID string) error {
	return m.setRuleEnabled(networkID, ruleID, false)
}

func (m *Manager) setRuleEnabled(networkID, ruleID string, enabled bool) error {
	r, err := m.store.GetRule(ruleID)
	if err != nil {
		return err
	}
	if r.NetworkID != networkID {
		return fmt.Errorf("firewall: rule %q does not belong to network %q", ruleID, networkID)
	}
	return m.store.SetRuleEnabled(ruleID, enabled)
}

// Apply compiles the firewall policy for net and either prints the nft script
// (dryRun=true) or pipes it to nft for application.
//
// leaseIPs maps instanceID → IP address (from network leases).
// labelIPs maps "key=value" → []IP (resolved from instance labels).
func (m *Manager) Apply(fw Firewall, net NetworkInfo, leaseIPs map[string]string, labelIPs map[string][]string, dryRun bool) (ApplyResult, error) {
	rules, err := m.store.ListRules(fw.NetworkID)
	if err != nil {
		return ApplyResult{}, err
	}

	script := Compile(CompileInput{
		Firewall: fw,
		Net:      net,
		Rules:    rules,
		LeaseIPs: leaseIPs,
		LabelIPs: labelIPs,
	})

	if dryRun {
		return ApplyResult{DryRun: true, Script: script, Applied: false}, nil
	}

	if err := ApplyScript(script); err != nil {
		_ = m.store.UpdateStatus(fw.NetworkID, StatusError)
		return ApplyResult{Script: script, Error: err.Error()}, err
	}

	_ = m.store.UpdateStatus(fw.NetworkID, StatusApplied)
	return ApplyResult{Script: script, Applied: true}, nil
}

// Reset deletes the nftables chain for the network and marks the firewall as pending.
func (m *Manager) Reset(networkID string) error {
	fw, err := m.store.Get(networkID)
	if err != nil {
		return err
	}
	chain := ChainName(networkID)
	if err := DeleteChain(chain); err != nil {
		return err
	}
	_ = m.store.UpdateStatus(fw.NetworkID, StatusPending)
	return nil
}

// Inspect returns the firewall configuration and all its rules.
func (m *Manager) Inspect(networkID string) (Firewall, []Rule, error) {
	fw, err := m.store.Get(networkID)
	if err != nil {
		return Firewall{}, nil, err
	}
	rules, err := m.store.ListRules(networkID)
	return fw, rules, err
}

// List returns all firewall records.
func (m *Manager) List() ([]Firewall, error) {
	return m.store.List()
}

// Delete removes the firewall record, rules, and nftables chain.
func (m *Manager) Delete(networkID string) error {
	_ = DeleteChain(ChainName(networkID)) // best-effort
	return m.store.Delete(networkID)
}

// ---- helpers ----------------------------------------------------------------

func defaultPolicy(mode string) string {
	switch mode {
	case ModePermissive:
		return ActionAllow
	default:
		return ActionDeny
	}
}

func defaultPriority(action string) int {
	switch action {
	case ActionAllow:
		return 100
	case ActionDeny, ActionReject:
		return 500
	default:
		return 100
	}
}

func newRuleID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "fwr_" + hex.EncodeToString(b)
}
