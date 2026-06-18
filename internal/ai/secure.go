package ai

import (
	"fmt"
	"strings"
	"time"
)

// ---- schema extension -------------------------------------------------------

// initSecureSchemaStmts returns the SQL statements for secure AI tables.
var initSecureSchemaStmts = []string{
	`CREATE TABLE IF NOT EXISTS ai_approval_gates (
		id            TEXT PRIMARY KEY,
		session_id    TEXT NOT NULL,
		agent_name    TEXT NOT NULL DEFAULT '',
		action        TEXT NOT NULL,
		resource      TEXT NOT NULL DEFAULT '',
		reason        TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'pending',
		reviewer_note TEXT NOT NULL DEFAULT '',
		created_at    TEXT NOT NULL,
		resolved_at   TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS ai_assumed_roles (
		id         TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		agent_id   TEXT NOT NULL,
		role_id    TEXT NOT NULL,
		role_name  TEXT NOT NULL,
		purpose    TEXT NOT NULL DEFAULT '',
		expires_at TEXT NOT NULL,
		revoked_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS ai_ledger (
		id               TEXT PRIMARY KEY,
		session_id       TEXT NOT NULL,
		agent_id         TEXT NOT NULL DEFAULT '',
		agent_name       TEXT NOT NULL DEFAULT '',
		human_principal  TEXT NOT NULL DEFAULT '',
		model            TEXT NOT NULL DEFAULT '',
		tool             TEXT NOT NULL DEFAULT '',
		action           TEXT NOT NULL DEFAULT '',
		resource         TEXT NOT NULL DEFAULT '',
		decision         TEXT NOT NULL DEFAULT '',
		reason           TEXT NOT NULL DEFAULT '',
		timestamp        TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS ai_policies (
		id               TEXT PRIMARY KEY,
		agent_id         TEXT NOT NULL,
		name             TEXT NOT NULL,
		effect           TEXT NOT NULL DEFAULT 'allow',
		actions_json     TEXT NOT NULL DEFAULT '[]',
		resources_json   TEXT NOT NULL DEFAULT '[]',
		conditions_json  TEXT NOT NULL DEFAULT '[]',
		require_approval INTEGER NOT NULL DEFAULT 0,
		created_at       TEXT NOT NULL,
		UNIQUE(agent_id, name)
	)`,
}

// ---- approval gates ---------------------------------------------------------

func (s *Store) CreateApprovalGate(g ApprovalGate) (ApprovalGate, error) {
	if g.ID == "" {
		g.ID = fmt.Sprintf("ag_%d", time.Now().UnixNano())
	}
	if g.CreatedAt == "" {
		g.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if g.Status == "" {
		g.Status = ApprovalPending
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_approval_gates (id, session_id, agent_name, action, resource, reason, status, reviewer_note, created_at, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.SessionID, g.AgentName, g.Action, g.Resource, g.Reason, g.Status, "", g.CreatedAt, "",
	)
	return g, err
}

func (s *Store) ResolveApprovalGate(id, status, reviewerNote string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE ai_approval_gates SET status=?, reviewer_note=?, resolved_at=? WHERE id=?`,
		status, reviewerNote, now, id,
	)
	return err
}

func (s *Store) ListApprovalGates(sessionID, status string) ([]ApprovalGate, error) {
	query := `SELECT id, session_id, agent_name, action, resource, reason, status, reviewer_note, created_at, resolved_at
	          FROM ai_approval_gates WHERE 1=1`
	var args []any
	if sessionID != "" {
		query += " AND session_id=?"
		args = append(args, sessionID)
	}
	if status != "" {
		query += " AND status=?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ApprovalGate
	for rows.Next() {
		var g ApprovalGate
		if err := rows.Scan(&g.ID, &g.SessionID, &g.AgentName, &g.Action, &g.Resource, &g.Reason, &g.Status, &g.ReviewerNote, &g.CreatedAt, &g.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ---- assumed roles ----------------------------------------------------------

func (s *Store) AssumeRole(sessionID, agentID, roleID, roleName, purpose string, ttl time.Duration) (AssumedRole, error) {
	now := time.Now().UTC()
	ar := AssumedRole{
		ID:        fmt.Sprintf("ar_%d", now.UnixNano()),
		SessionID: sessionID,
		AgentID:   agentID,
		RoleID:    roleID,
		RoleName:  roleName,
		Purpose:   purpose,
		ExpiresAt: now.Add(ttl).Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_assumed_roles (id, session_id, agent_id, role_id, role_name, purpose, expires_at, revoked_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ar.ID, ar.SessionID, ar.AgentID, ar.RoleID, ar.RoleName, ar.Purpose, ar.ExpiresAt, "", ar.CreatedAt,
	)
	return ar, err
}

func (s *Store) RevokeAssumedRole(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE ai_assumed_roles SET revoked_at=? WHERE id=?`, now, id)
	return err
}

func (s *Store) ListAssumedRoles(sessionID string) ([]AssumedRole, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, agent_id, role_id, role_name, purpose, expires_at, revoked_at, created_at
		 FROM ai_assumed_roles WHERE session_id=? ORDER BY created_at DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssumedRole
	for rows.Next() {
		var ar AssumedRole
		if err := rows.Scan(&ar.ID, &ar.SessionID, &ar.AgentID, &ar.RoleID, &ar.RoleName, &ar.Purpose, &ar.ExpiresAt, &ar.RevokedAt, &ar.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	return out, rows.Err()
}

// ---- immutable ledger -------------------------------------------------------

// AppendLedger writes an immutable audit record. INSERT only — no UPDATE/DELETE.
func (s *Store) AppendLedger(e LedgerEntry) (LedgerEntry, error) {
	if e.ID == "" {
		e.ID = fmt.Sprintf("le_%d", time.Now().UnixNano())
	}
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_ledger (id, session_id, agent_id, agent_name, human_principal, model, tool, action, resource, decision, reason, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.SessionID, e.AgentID, e.AgentName, e.HumanPrincipal, e.Model,
		e.Tool, e.Action, e.Resource, e.Decision, e.Reason, e.Timestamp,
	)
	return e, err
}

func (s *Store) QueryLedger(sessionID string, limit int) ([]LedgerEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows interface{ Next() bool; Scan(dest ...any) error; Close() error; Err() error }
	var err error
	if sessionID != "" {
		rows, err = s.db.Query(
			`SELECT id, session_id, agent_id, agent_name, human_principal, model, tool, action, resource, decision, reason, timestamp
			 FROM ai_ledger WHERE session_id=? ORDER BY timestamp DESC LIMIT ?`,
			sessionID, limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, session_id, agent_id, agent_name, human_principal, model, tool, action, resource, decision, reason, timestamp
			 FROM ai_ledger ORDER BY timestamp DESC LIMIT ?`,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.AgentID, &e.AgentName, &e.HumanPrincipal, &e.Model, &e.Tool, &e.Action, &e.Resource, &e.Decision, &e.Reason, &e.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- AI policy engine -------------------------------------------------------

func (s *Store) CreateAIPolicy(p AIPolicy) (AIPolicy, error) {
	if p.ID == "" {
		p.ID = fmt.Sprintf("aip_%d", time.Now().UnixNano())
	}
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	actJSON := marshalStringSlice(p.Actions)
	resJSON := marshalStringSlice(p.Resources)
	condJSON := "{}"
	ra := 0
	if p.RequireApproval {
		ra = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_policies (id, agent_id, name, effect, actions_json, resources_json, conditions_json, require_approval, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.AgentID, p.Name, p.Effect, actJSON, resJSON, condJSON, ra, p.CreatedAt,
	)
	return p, err
}

func (s *Store) ListAIPolicies(agentID string) ([]AIPolicy, error) {
	rows, err := s.db.Query(
		`SELECT id, agent_id, name, effect, actions_json, resources_json, conditions_json, require_approval, created_at
		 FROM ai_policies WHERE agent_id=? ORDER BY name`, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AIPolicy
	for rows.Next() {
		var p AIPolicy
		var actJSON, resJSON, condJSON string
		var ra int
		if err := rows.Scan(&p.ID, &p.AgentID, &p.Name, &p.Effect, &actJSON, &resJSON, &condJSON, &ra, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.Actions = unmarshalStringSlice(actJSON)
		p.Resources = unmarshalStringSlice(resJSON)
		p.RequireApproval = ra != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

// EvaluateAIPolicy checks whether agentID may perform action on resource.
// Returns (allowed bool, requireApproval bool, err).
// deny policies take precedence over allow policies.
func (s *Store) EvaluateAIPolicy(agentID, action, resource string) (allowed, requireApproval bool, err error) {
	policies, err := s.ListAIPolicies(agentID)
	if err != nil {
		return false, false, err
	}
	for _, p := range policies {
		if !actionMatches(p.Actions, action) {
			continue
		}
		if !resourceMatches(p.Resources, resource) {
			continue
		}
		if p.Effect == "deny" {
			return false, false, fmt.Errorf("ai policy %q denies %s on %s", p.Name, action, resource)
		}
		if p.Effect == "allow" {
			return true, p.RequireApproval, nil
		}
	}
	// Default deny if no matching allow policy.
	return false, false, nil
}

func actionMatches(actions []string, action string) bool {
	for _, a := range actions {
		if a == "*" || a == action || (strings.HasSuffix(a, "*") && strings.HasPrefix(action, strings.TrimSuffix(a, "*"))) {
			return true
		}
	}
	return false
}

func resourceMatches(resources []string, resource string) bool {
	if len(resources) == 0 {
		return true
	}
	for _, r := range resources {
		if r == "*" || r == resource || (strings.HasSuffix(r, "*") && strings.HasPrefix(resource, strings.TrimSuffix(r, "*"))) {
			return true
		}
	}
	return false
}

func marshalStringSlice(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range ss {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(s, `"`, `\"`))
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

func unmarshalStringSlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return nil
	}
	s = strings.Trim(s, "[]")
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
