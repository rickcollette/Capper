package mcpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store persists MCP servers, tools, tool invocations, and approvals.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over an open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the MCP tables. Safe to call repeatedly.
func (s *Store) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			name TEXT NOT NULL,
			runtime TEXT NOT NULL,
			transport TEXT NOT NULL DEFAULT 'streamable-http',
			endpoint TEXT NOT NULL DEFAULT '',
			package_id TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			command_json TEXT NOT NULL DEFAULT '[]',
			version TEXT NOT NULL DEFAULT '1',
			status TEXT NOT NULL DEFAULT 'created',
			default_iam_action TEXT NOT NULL DEFAULT '',
			memory_bytes INTEGER NOT NULL DEFAULT 536870912,
			cpu_units INTEGER NOT NULL DEFAULT 500,
			timeout_ms INTEGER NOT NULL DEFAULT 60000,
			concurrency INTEGER NOT NULL DEFAULT 10,
			min_scale INTEGER NOT NULL DEFAULT 0,
			max_scale INTEGER NOT NULL DEFAULT 5,
			isolation TEXT NOT NULL DEFAULT 'microvm',
			approval_policy TEXT NOT NULL DEFAULT 'dangerous-only',
			env_json TEXT NOT NULL DEFAULT '{}',
			labels_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(project, name)
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_tools (
			id TEXT PRIMARY KEY,
			mcp_server_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			input_schema_json TEXT NOT NULL DEFAULT '{}',
			output_schema_json TEXT NOT NULL DEFAULT '{}',
			iam_action TEXT NOT NULL DEFAULT '',
			resource_pattern TEXT NOT NULL DEFAULT '',
			read_only INTEGER NOT NULL DEFAULT 0,
			approval_required INTEGER NOT NULL DEFAULT 0,
			dangerous INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(mcp_server_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_tool_invocations (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			mcp_server_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			session_id TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			principal TEXT NOT NULL DEFAULT '',
			request_id TEXT NOT NULL DEFAULT '',
			arguments_hash TEXT NOT NULL DEFAULT '',
			decision TEXT NOT NULL DEFAULT '',
			approval_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT NOT NULL DEFAULT '',
			ended_at TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			result_ref TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_approvals (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			mcp_server_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			principal TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			invocation_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			decided_by TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			decided_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mcp_tools_server ON mcp_tools(mcp_server_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mcp_tool_inv_server ON mcp_tool_invocations(mcp_server_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_mcp_approvals_status ON mcp_approvals(status, project)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("mcpserver: schema: %w", err)
		}
	}
	return nil
}

func nowTS() string { return time.Now().UTC().Format(time.RFC3339) }
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
func encJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
func decMap(s string) map[string]string {
	m := map[string]string{}
	if s != "" {
		_ = json.Unmarshal([]byte(s), &m)
	}
	return m
}
func decStrSlice(s string) []string {
	var out []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

// ---- servers ---------------------------------------------------------------

// CreateServer inserts a new MCP server with defaults.
func (s *Store) CreateServer(srv Server) (Server, error) {
	if srv.ID == "" {
		srv.ID = "mcp_" + uuid.NewString()
	}
	ts := nowTS()
	srv.CreatedAt, srv.UpdatedAt = ts, ts
	if srv.Version == "" {
		srv.Version = "1"
	}
	if srv.Status == "" {
		srv.Status = StatusCreated
	}
	if srv.Transport == "" {
		srv.Transport = "streamable-http"
	}
	if srv.ApprovalPolicy == "" {
		srv.ApprovalPolicy = "dangerous-only"
	}
	if srv.Isolation == "" {
		srv.Isolation = "microvm"
	}
	_, err := s.db.Exec(`INSERT INTO mcp_servers
		(id, project, name, runtime, transport, endpoint, package_id, image, command_json, version,
		 status, default_iam_action, memory_bytes, cpu_units, timeout_ms, concurrency, min_scale,
		 max_scale, isolation, approval_policy, env_json, labels_json, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		srv.ID, srv.Project, srv.Name, srv.Runtime, srv.Transport, srv.Endpoint, srv.PackageID,
		srv.Image, encJSON(srv.Command), srv.Version, srv.Status, srv.DefaultIAMAction, srv.MemoryBytes,
		srv.CPUUnits, srv.TimeoutMS, srv.Concurrency, srv.MinScale, srv.MaxScale, srv.Isolation,
		srv.ApprovalPolicy, encJSON(srv.Env), encJSON(srv.Labels), srv.CreatedAt, srv.UpdatedAt)
	if err != nil {
		return Server{}, err
	}
	return srv, nil
}

const srvCols = `id, project, name, runtime, transport, endpoint, package_id, image, command_json,
	version, status, default_iam_action, memory_bytes, cpu_units, timeout_ms, concurrency, min_scale,
	max_scale, isolation, approval_policy, env_json, labels_json, created_at, updated_at`

func scanServer(row interface{ Scan(...any) error }) (Server, error) {
	var srv Server
	var cmd, env, labels string
	if err := row.Scan(&srv.ID, &srv.Project, &srv.Name, &srv.Runtime, &srv.Transport, &srv.Endpoint,
		&srv.PackageID, &srv.Image, &cmd, &srv.Version, &srv.Status, &srv.DefaultIAMAction,
		&srv.MemoryBytes, &srv.CPUUnits, &srv.TimeoutMS, &srv.Concurrency, &srv.MinScale, &srv.MaxScale,
		&srv.Isolation, &srv.ApprovalPolicy, &env, &labels, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
		return Server{}, err
	}
	srv.Command = decStrSlice(cmd)
	srv.Env = decMap(env)
	srv.Labels = decMap(labels)
	return srv, nil
}

// GetServer returns a server by ID.
func (s *Store) GetServer(id string) (Server, error) {
	return scanServer(s.db.QueryRow(`SELECT `+srvCols+` FROM mcp_servers WHERE id=?`, id))
}

// ListServers returns servers, optionally scoped to a project.
func (s *Store) ListServers(project string) ([]Server, error) {
	q := `SELECT ` + srvCols + ` FROM mcp_servers`
	var args []any
	if project != "" {
		q += " WHERE project=?"
		args = append(args, project)
	}
	q += " ORDER BY name"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// DeleteServer removes a server and its tools/invocations/approvals.
func (s *Store) DeleteServer(id string) error {
	for _, q := range []string{
		`DELETE FROM mcp_tools WHERE mcp_server_id=?`,
		`DELETE FROM mcp_tool_invocations WHERE mcp_server_id=?`,
		`DELETE FROM mcp_approvals WHERE mcp_server_id=?`,
		`DELETE FROM mcp_servers WHERE id=?`,
	} {
		if _, err := s.db.Exec(q, id); err != nil {
			return err
		}
	}
	return nil
}

// ---- tools -----------------------------------------------------------------

// UpsertTool inserts or updates a tool by (server, name).
func (s *Store) UpsertTool(t Tool) (Tool, error) {
	if t.ID == "" {
		t.ID = "mcptool_" + uuid.NewString()
	}
	ts := nowTS()
	if t.CreatedAt == "" {
		t.CreatedAt = ts
	}
	t.UpdatedAt = ts
	_, err := s.db.Exec(`INSERT INTO mcp_tools
		(id, mcp_server_id, name, description, input_schema_json, output_schema_json, iam_action,
		 resource_pattern, read_only, approval_required, dangerous, enabled, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(mcp_server_id, name) DO UPDATE SET
			description=excluded.description, input_schema_json=excluded.input_schema_json,
			output_schema_json=excluded.output_schema_json, iam_action=excluded.iam_action,
			resource_pattern=excluded.resource_pattern, read_only=excluded.read_only,
			approval_required=excluded.approval_required, dangerous=excluded.dangerous,
			enabled=excluded.enabled, updated_at=excluded.updated_at`,
		t.ID, t.ServerID, t.Name, t.Description, jsonOr(t.InputSchemaJSON), jsonOr(t.OutputSchemaJSON),
		t.IAMAction, t.ResourcePattern, b2i(t.ReadOnly), b2i(t.ApprovalRequired), b2i(t.Dangerous),
		b2i(t.Enabled), t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return Tool{}, err
	}
	return s.GetTool(t.ServerID, t.Name)
}

func jsonOr(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

func scanTool(row interface{ Scan(...any) error }) (Tool, error) {
	var t Tool
	var ro, ar, dg, en int
	if err := row.Scan(&t.ID, &t.ServerID, &t.Name, &t.Description, &t.InputSchemaJSON,
		&t.OutputSchemaJSON, &t.IAMAction, &t.ResourcePattern, &ro, &ar, &dg, &en,
		&t.CreatedAt, &t.UpdatedAt); err != nil {
		return Tool{}, err
	}
	t.ReadOnly, t.ApprovalRequired, t.Dangerous, t.Enabled = ro == 1, ar == 1, dg == 1, en == 1
	return t, nil
}

const toolCols = `id, mcp_server_id, name, description, input_schema_json, output_schema_json,
	iam_action, resource_pattern, read_only, approval_required, dangerous, enabled, created_at, updated_at`

// GetTool returns a tool by server + name.
func (s *Store) GetTool(serverID, name string) (Tool, error) {
	return scanTool(s.db.QueryRow(`SELECT `+toolCols+` FROM mcp_tools WHERE mcp_server_id=? AND name=?`, serverID, name))
}

// ListTools returns the tools of a server.
func (s *Store) ListTools(serverID string) ([]Tool, error) {
	rows, err := s.db.Query(`SELECT `+toolCols+` FROM mcp_tools WHERE mcp_server_id=? ORDER BY name`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tool
	for rows.Next() {
		t, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---- invocations -----------------------------------------------------------

// RecordInvocation inserts a tool invocation row.
func (s *Store) RecordInvocation(inv ToolInvocation) (ToolInvocation, error) {
	if inv.ID == "" {
		inv.ID = "mcpinv_" + uuid.NewString()
	}
	if inv.CreatedAt == "" {
		inv.CreatedAt = nowTS()
	}
	_, err := s.db.Exec(`INSERT INTO mcp_tool_invocations
		(id, project, mcp_server_id, tool_name, session_id, agent_id, principal, request_id,
		 arguments_hash, decision, approval_id, status, started_at, ended_at, duration_ms, error, result_ref, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		inv.ID, inv.Project, inv.ServerID, inv.ToolName, inv.SessionID, inv.AgentID, inv.Principal,
		inv.RequestID, inv.ArgumentsHash, inv.Decision, inv.ApprovalID, inv.Status, inv.StartedAt,
		inv.EndedAt, inv.DurationMS, inv.Error, inv.Result, inv.CreatedAt)
	if err != nil {
		return ToolInvocation{}, err
	}
	return inv, nil
}

// FinishInvocation updates an invocation's terminal fields.
func (s *Store) FinishInvocation(id, status string, durationMS int64, errMsg, result string) error {
	_, err := s.db.Exec(`UPDATE mcp_tool_invocations SET status=?, ended_at=?, duration_ms=?, error=?, result_ref=? WHERE id=?`,
		status, nowTS(), durationMS, errMsg, result, id)
	return err
}

// ListInvocations returns invocations for a server, newest first.
func (s *Store) ListInvocations(serverID string, limit int) ([]ToolInvocation, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, project, mcp_server_id, tool_name, session_id, agent_id, principal,
		request_id, arguments_hash, decision, approval_id, status, started_at, ended_at, duration_ms,
		error, result_ref, created_at FROM mcp_tool_invocations WHERE mcp_server_id=?
		ORDER BY created_at DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ToolInvocation
	for rows.Next() {
		var inv ToolInvocation
		if err := rows.Scan(&inv.ID, &inv.Project, &inv.ServerID, &inv.ToolName, &inv.SessionID,
			&inv.AgentID, &inv.Principal, &inv.RequestID, &inv.ArgumentsHash, &inv.Decision,
			&inv.ApprovalID, &inv.Status, &inv.StartedAt, &inv.EndedAt, &inv.DurationMS, &inv.Error,
			&inv.Result, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// ---- approvals -------------------------------------------------------------

// CreateApproval inserts a pending approval.
func (s *Store) CreateApproval(a Approval) (Approval, error) {
	if a.ID == "" {
		a.ID = "mcpappr_" + uuid.NewString()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = nowTS()
	}
	if a.Status == "" {
		a.Status = ApprovalPending
	}
	_, err := s.db.Exec(`INSERT INTO mcp_approvals
		(id, project, mcp_server_id, tool_name, principal, agent_id, invocation_id, status, decided_by, reason, created_at, decided_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Project, a.ServerID, a.ToolName, a.Principal, a.AgentID, a.InvocationID, a.Status,
		a.DecidedBy, a.Reason, a.CreatedAt, a.DecidedAt)
	if err != nil {
		return Approval{}, err
	}
	return a, nil
}

func scanApproval(row interface{ Scan(...any) error }) (Approval, error) {
	var a Approval
	err := row.Scan(&a.ID, &a.Project, &a.ServerID, &a.ToolName, &a.Principal, &a.AgentID,
		&a.InvocationID, &a.Status, &a.DecidedBy, &a.Reason, &a.CreatedAt, &a.DecidedAt)
	return a, err
}

const apprCols = `id, project, mcp_server_id, tool_name, principal, agent_id, invocation_id, status,
	decided_by, reason, created_at, decided_at`

// GetApproval returns an approval by ID.
func (s *Store) GetApproval(id string) (Approval, error) {
	return scanApproval(s.db.QueryRow(`SELECT `+apprCols+` FROM mcp_approvals WHERE id=?`, id))
}

// ListApprovals returns approvals filtered by status (empty = all).
func (s *Store) ListApprovals(status string) ([]Approval, error) {
	q := `SELECT ` + apprCols + ` FROM mcp_approvals`
	var args []any
	if status != "" {
		q += " WHERE status=?"
		args = append(args, status)
	}
	q += " ORDER BY created_at DESC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DecideApproval marks an approval approved/denied.
func (s *Store) DecideApproval(id, status, decidedBy, reason string) error {
	_, err := s.db.Exec(`UPDATE mcp_approvals SET status=?, decided_by=?, reason=?, decided_at=?
		WHERE id=? AND status='pending'`, status, decidedBy, reason, nowTS(), id)
	return err
}
