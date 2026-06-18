package billing

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// UsageRecord tracks resource usage for billing/quota purposes.
type UsageRecord struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Resource   string `json:"resource"` // "instance", "storage", "network"
	ResourceID string `json:"resourceId"`
	Units      int64  `json:"units"`    // seconds, bytes, etc.
	UnitType   string `json:"unitType"` // "seconds", "bytes"
	RecordedAt string `json:"recordedAt"`
}

// Quota defines a resource limit for a project.
type Quota struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Resource string `json:"resource"`
	Limit    int64  `json:"limit"`
	Used     int64  `json:"used"`
}

// Store persists billing and quota data.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS billing_usage (
		id          TEXT PRIMARY KEY,
		project     TEXT NOT NULL,
		resource    TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		units       INTEGER NOT NULL DEFAULT 0,
		unit_type   TEXT NOT NULL DEFAULT 'seconds',
		recorded_at TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS project_quotas (
		id       TEXT PRIMARY KEY,
		project  TEXT NOT NULL,
		resource TEXT NOT NULL,
		limit_v  INTEGER NOT NULL DEFAULT 0,
		UNIQUE(project, resource)
	)`); err != nil {
		return err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS governance_policies (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		project     TEXT NOT NULL DEFAULT '',
		resource    TEXT NOT NULL,
		action      TEXT NOT NULL,
		effect      TEXT NOT NULL,
		condition_v TEXT NOT NULL DEFAULT '',
		priority    INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL,
		UNIQUE(name, project)
	)`); err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS account_quotas (
		account_id TEXT NOT NULL,
		resource   TEXT NOT NULL,
		limit_v    INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY(account_id, resource)
	)`)
	return err
}

// Manager handles billing and quota enforcement.
type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager { return &Manager{store: s} }

func (m *Manager) GetQuota(project, resource string) (Quota, error) {
	row := m.store.db.QueryRow(
		`SELECT id, project, resource, limit_v FROM project_quotas WHERE project=? AND resource=?`,
		project, resource,
	)
	var q Quota
	if err := row.Scan(&q.ID, &q.Project, &q.Resource, &q.Limit); err != nil {
		return Quota{}, err
	}
	return q, nil
}

func (m *Manager) SetQuota(project, resource string, limit int64) error {
	_, err := m.store.db.Exec(
		`INSERT OR REPLACE INTO project_quotas (id, project, resource, limit_v)
		 VALUES (lower(hex(randomblob(8))), ?, ?, ?)`,
		project, resource, limit,
	)
	return err
}

func (m *Manager) ListQuotas(project string) ([]Quota, error) {
	rows, err := m.store.db.Query(
		`SELECT id, project, resource, limit_v FROM project_quotas WHERE project=?`, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Quota
	for rows.Next() {
		var q Quota
		if err := rows.Scan(&q.ID, &q.Project, &q.Resource, &q.Limit); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ProjectTypeQuota limits how many instances of a specific type a project may run.
type ProjectTypeQuota struct {
	Project      string `json:"project"`
	InstanceType string `json:"instanceType"`
	MaxInstances int64  `json:"maxInstances"`
}

// RecordUsage inserts a usage record.
func (m *Manager) RecordUsage(project, resource, resourceID, unitType string, units int64) error {
	_, err := m.store.db.Exec(
		`INSERT INTO billing_usage (id, project, resource, resource_id, units, unit_type, recorded_at)
		 VALUES (lower(hex(randomblob(8))), ?, ?, ?, ?, ?, datetime('now'))`,
		project, resource, resourceID, units, unitType,
	)
	return err
}

// CountUsage returns the total units consumed for a resource in a project.
func (m *Manager) CountUsage(project, resource string) (int64, error) {
	row := m.store.db.QueryRow(
		`SELECT COALESCE(SUM(units), 0) FROM billing_usage WHERE project=? AND resource=?`,
		project, resource,
	)
	var total int64
	return total, row.Scan(&total)
}

// ReleaseUsage balances previous usage records for a specific resource ID.
// Usage remains auditable while aggregate quota accounting returns to zero.
func (m *Manager) ReleaseUsage(project, resource, resourceID string) error {
	row := m.store.db.QueryRow(
		`SELECT COALESCE(SUM(units), 0) FROM billing_usage WHERE project=? AND resource=? AND resource_id=?`,
		project, resource, resourceID,
	)
	var total int64
	if err := row.Scan(&total); err != nil {
		return err
	}
	if total <= 0 {
		return nil
	}
	return m.RecordUsage(project, resource, resourceID, "release", -total)
}

// CheckQuota returns an error when current usage meets or exceeds the quota limit.
// Returns nil when no quota has been set (open policy).
func (m *Manager) CheckQuota(project, resource string) error {
	q, err := m.GetQuota(project, resource)
	if err != nil {
		return nil // no quota row → open
	}
	if q.Limit <= 0 {
		return nil // limit 0 = unlimited
	}
	used, err := m.CountUsage(project, resource)
	if err != nil {
		// Fail open so a transient usage-table error can't block all writes, but
		// make it observable — a silent allow here disables the quota entirely.
		slog.Warn("billing: quota check failed open (usage count error)",
			"project", project, "resource", resource, "limit", q.Limit, "err", err)
		return nil
	}
	if used >= q.Limit {
		return fmt.Errorf("quota exceeded: project %q has used %d/%d %s", project, used, q.Limit, resource)
	}
	return nil
}

// ---- governance policy engine (Block 18) ------------------------------------

// PolicyRule defines a governance constraint applied across the platform.
type PolicyRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`             // empty = global
	Resource  string `json:"resource"`            // e.g. "instance", "network", "secret", "*"
	Action    string `json:"action"`              // e.g. "create", "delete", "*"
	Effect    string `json:"effect"`              // "allow" or "deny"
	Condition string `json:"condition,omitempty"` // simple expression: "label.env=prod"
	Priority  int    `json:"priority"`
	CreatedAt string `json:"createdAt"`
}

// AddGovernancePolicy registers a governance rule.
func (m *Manager) AddGovernancePolicy(name, project, resource, action, effect, condition string, priority int) PolicyRule {
	rule := PolicyRule{
		ID:   fmt.Sprintf("pol_%d", time.Now().UnixNano()),
		Name: name, Project: project, Resource: resource, Action: action,
		Effect: effect, Condition: condition, Priority: priority,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, _ = m.store.db.Exec(
		`INSERT OR REPLACE INTO governance_policies
		 (id, name, project, resource, action, effect, condition_v, priority, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.Project, rule.Resource, rule.Action, rule.Effect, rule.Condition, rule.Priority, rule.CreatedAt,
	)
	return rule
}

// EvaluateGovernance returns (allowed, matchedRule) for a given action.
// Returns (true, "") if no deny rules match.
func (m *Manager) EvaluateGovernance(project, resource, action string, labels map[string]string) (bool, string) {
	rows, err := m.store.db.Query(
		`SELECT id, name, project, resource, action, effect, condition_v, priority, created_at
		 FROM governance_policies
		 WHERE (project='' OR project=?) AND (resource='*' OR resource=?) AND (action='*' OR action=?)
		 ORDER BY priority DESC, created_at ASC`,
		project, resource, action,
	)
	if err != nil {
		return true, ""
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPolicyRule(rows)
		if err != nil {
			continue
		}
		if p.Condition != "" && !evalCondition(p.Condition, labels) {
			continue
		}
		if strings.EqualFold(p.Effect, "deny") {
			return false, p.Name
		}
	}
	return true, ""
}

// ListGovernancePolicies returns policies applicable to a project.
func (m *Manager) ListGovernancePolicies(project string) []PolicyRule {
	rows, err := m.store.db.Query(
		`SELECT id, name, project, resource, action, effect, condition_v, priority, created_at
		 FROM governance_policies WHERE project='' OR project=? ORDER BY priority DESC, name`,
		project,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []PolicyRule
	for rows.Next() {
		p, err := scanPolicyRule(rows)
		if err == nil {
			out = append(out, p)
		}
	}
	return out
}

func evalCondition(condition string, labels map[string]string) bool {
	// Simple "key=value" label match.
	if len(condition) > 6 && condition[:6] == "label." {
		rest := condition[6:]
		k, v, ok := strings.Cut(rest, "=")
		if ok {
			return labels[k] == v
		}
	}
	return false
}

func scanPolicyRule(s interface{ Scan(dest ...any) error }) (PolicyRule, error) {
	var p PolicyRule
	return p, s.Scan(&p.ID, &p.Name, &p.Project, &p.Resource, &p.Action, &p.Effect, &p.Condition, &p.Priority, &p.CreatedAt)
}

// ---- quota by account (Block 17) --------------------------------------------

// AccountQuota limits resources for a specific org account.
type AccountQuota struct {
	AccountID string `json:"accountId"`
	Resource  string `json:"resource"`
	Limit     int64  `json:"limit"`
}

// SetAccountQuota records a resource limit for an account.
func (m *Manager) SetAccountQuota(accountID, resource string, limit int64) {
	_, _ = m.store.db.Exec(
		`INSERT OR REPLACE INTO account_quotas (account_id, resource, limit_v) VALUES (?, ?, ?)`,
		accountID, resource, limit,
	)
}

// CheckAccountQuota returns an error when the account usage meets or exceeds its quota.
func (m *Manager) CheckAccountQuota(accountID, resource string) error {
	row := m.store.db.QueryRow(
		`SELECT account_id, resource, limit_v FROM account_quotas WHERE account_id=? AND resource=?`,
		accountID, resource,
	)
	var q AccountQuota
	if err := row.Scan(&q.AccountID, &q.Resource, &q.Limit); err != nil {
		return nil
	}
	if q.Limit <= 0 {
		return nil
	}
	used, _ := m.CountUsage(accountID, resource)
	if used >= q.Limit {
		return fmt.Errorf("account %q quota exceeded: %d/%d %s", accountID, used, q.Limit, resource)
	}
	return nil
}
