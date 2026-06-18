package firewall

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// InitSchema creates the firewall tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS firewalls (
			network_id            TEXT PRIMARY KEY,
			network_name          TEXT NOT NULL,
			mode                  TEXT NOT NULL DEFAULT 'strict',
			backend               TEXT NOT NULL DEFAULT 'nftables',
			default_forward_policy TEXT NOT NULL DEFAULT 'deny',
			default_ingress_policy TEXT NOT NULL DEFAULT 'deny',
			default_egress_policy  TEXT NOT NULL DEFAULT 'deny',
			allow_dns             INTEGER NOT NULL DEFAULT 1,
			allow_established     INTEGER NOT NULL DEFAULT 1,
			nat_enabled            INTEGER NOT NULL DEFAULT 0,
			status                TEXT NOT NULL DEFAULT 'pending',
			last_applied_at        TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS firewall_rules (
			id          TEXT PRIMARY KEY,
			network_id  TEXT NOT NULL,
			priority    INTEGER NOT NULL DEFAULT 100,
			enabled     INTEGER NOT NULL DEFAULT 1,
			action      TEXT NOT NULL,
			direction   TEXT NOT NULL DEFAULT 'forward',
			from_type   TEXT NOT NULL DEFAULT 'any',
			from_key    TEXT NOT NULL DEFAULT '',
			from_value  TEXT NOT NULL DEFAULT '',
			to_type     TEXT NOT NULL DEFAULT 'any',
			to_key      TEXT NOT NULL DEFAULT '',
			to_value    TEXT NOT NULL DEFAULT '',
			protocol    TEXT NOT NULL DEFAULT 'any',
			ports_json  TEXT NOT NULL DEFAULT '[]',
			description TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_firewall_rules_network_priority
			ON firewall_rules(network_id, priority);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("firewall.InitSchema: %w", err)
		}
	}
	return nil
}

// Store provides CRUD access to firewall configuration.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Insert creates a new Firewall record. Fails if one already exists for the network.
func (s *Store) Insert(fw Firewall) error {
	_, err := s.db.Exec(
		`INSERT INTO firewalls
			(network_id, network_name, mode, backend,
			 default_forward_policy, default_ingress_policy, default_egress_policy,
			 allow_dns, allow_established, nat_enabled, status, last_applied_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		fw.NetworkID, fw.NetworkName, fw.Mode, fw.Backend,
		fw.DefaultForwardPolicy, fw.DefaultIngressPolicy, fw.DefaultEgressPolicy,
		boolInt(fw.AllowDNS), boolInt(fw.AllowEstablished), boolInt(fw.NATEnabled),
		fw.Status, nullableStr(fw.LastAppliedAt),
	)
	if err != nil {
		return fmt.Errorf("firewall: insert: %w", err)
	}
	return nil
}

// Get returns the Firewall for the given networkID.
func (s *Store) Get(networkID string) (Firewall, error) {
	row := s.db.QueryRow(
		`SELECT network_id, network_name, mode, backend,
			default_forward_policy, default_ingress_policy, default_egress_policy,
			allow_dns, allow_established, nat_enabled, status,
			COALESCE(last_applied_at, '')
		FROM firewalls WHERE network_id = ?`, networkID)
	return scanFirewall(row)
}

// List returns all firewall records.
func (s *Store) List() ([]Firewall, error) {
	rows, err := s.db.Query(
		`SELECT network_id, network_name, mode, backend,
			default_forward_policy, default_ingress_policy, default_egress_policy,
			allow_dns, allow_established, nat_enabled, status,
			COALESCE(last_applied_at, '')
		FROM firewalls ORDER BY network_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Firewall
	for rows.Next() {
		fw, err := scanFirewall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fw)
	}
	return out, rows.Err()
}

// UpdateStatus sets the status and, if applied, records the timestamp.
func (s *Store) UpdateStatus(networkID, status string) error {
	var ts any
	if status == StatusApplied {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`UPDATE firewalls SET status=?, last_applied_at=? WHERE network_id=?`,
		status, ts, networkID)
	return err
}

// Delete removes the firewall and all its rules.
func (s *Store) Delete(networkID string) error {
	if _, err := s.db.Exec(`DELETE FROM firewall_rules WHERE network_id=?`, networkID); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM firewalls WHERE network_id=?`, networkID)
	return err
}

// InsertRule adds a Rule to the store.
func (s *Store) InsertRule(r Rule) error {
	ports, _ := json.Marshal(r.Ports)
	_, err := s.db.Exec(
		`INSERT INTO firewall_rules
			(id, network_id, priority, enabled, action, direction,
			 from_type, from_key, from_value,
			 to_type, to_key, to_value,
			 protocol, ports_json, description, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.NetworkID, r.Priority, boolInt(r.Enabled), r.Action, r.Direction,
		r.From.Type, r.From.Key, r.From.Value,
		r.To.Type, r.To.Key, r.To.Value,
		r.Protocol, string(ports), r.Description, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("firewall: insert rule: %w", err)
	}
	return nil
}

// GetRule returns a single rule by ID.
func (s *Store) GetRule(ruleID string) (Rule, error) {
	row := s.db.QueryRow(
		`SELECT id, network_id, priority, enabled, action, direction,
			from_type, from_key, from_value,
			to_type, to_key, to_value,
			protocol, ports_json, description, created_at
		FROM firewall_rules WHERE id=?`, ruleID)
	return scanRule(row)
}

// ListRules returns all rules for a network, ordered by priority.
func (s *Store) ListRules(networkID string) ([]Rule, error) {
	rows, err := s.db.Query(
		`SELECT id, network_id, priority, enabled, action, direction,
			from_type, from_key, from_value,
			to_type, to_key, to_value,
			protocol, ports_json, description, created_at
		FROM firewall_rules WHERE network_id=? ORDER BY priority, created_at`, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRule removes a rule.
func (s *Store) DeleteRule(ruleID string) error {
	res, err := s.db.Exec(`DELETE FROM firewall_rules WHERE id=?`, ruleID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("firewall: rule %q not found", ruleID)
	}
	return nil
}

// SetRuleEnabled toggles a rule's enabled state.
func (s *Store) SetRuleEnabled(ruleID string, enabled bool) error {
	res, err := s.db.Exec(`UPDATE firewall_rules SET enabled=? WHERE id=?`, boolInt(enabled), ruleID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("firewall: rule %q not found", ruleID)
	}
	return nil
}

// ---- scanner helpers --------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanFirewall(s scanner) (Firewall, error) {
	var fw Firewall
	var allowDNS, allowEstablished, natEnabled int
	err := s.Scan(
		&fw.NetworkID, &fw.NetworkName, &fw.Mode, &fw.Backend,
		&fw.DefaultForwardPolicy, &fw.DefaultIngressPolicy, &fw.DefaultEgressPolicy,
		&allowDNS, &allowEstablished, &natEnabled,
		&fw.Status, &fw.LastAppliedAt,
	)
	if err != nil {
		return Firewall{}, fmt.Errorf("firewall: scan: %w", err)
	}
	fw.AllowDNS = allowDNS != 0
	fw.AllowEstablished = allowEstablished != 0
	fw.NATEnabled = natEnabled != 0
	return fw, nil
}

func scanRule(s scanner) (Rule, error) {
	var r Rule
	var enabled int
	var portsJSON string
	err := s.Scan(
		&r.ID, &r.NetworkID, &r.Priority, &enabled, &r.Action, &r.Direction,
		&r.From.Type, &r.From.Key, &r.From.Value,
		&r.To.Type, &r.To.Key, &r.To.Value,
		&r.Protocol, &portsJSON, &r.Description, &r.CreatedAt,
	)
	if err != nil {
		return Rule{}, fmt.Errorf("firewall: scan rule: %w", err)
	}
	r.Enabled = enabled != 0
	if err := json.Unmarshal([]byte(portsJSON), &r.Ports); err != nil {
		r.Ports = nil
	}
	return r, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
