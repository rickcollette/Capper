package ai

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ai_agents (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			project       TEXT NOT NULL DEFAULT 'default',
			model         TEXT NOT NULL DEFAULT '',
			owner         TEXT NOT NULL DEFAULT '',
			role_template TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL DEFAULT 'active',
			created_at    TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS ai_sessions (
			id         TEXT PRIMARY KEY,
			agent_id   TEXT NOT NULL,
			project    TEXT NOT NULL DEFAULT 'default',
			principal  TEXT NOT NULL DEFAULT '',
			model      TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'active',
			started_at TEXT NOT NULL,
			ended_at   TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS ai_mcp_servers (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			project    TEXT NOT NULL DEFAULT 'default',
			endpoint   TEXT NOT NULL DEFAULT '',
			tools_json TEXT NOT NULL DEFAULT '',
			iam_action TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS ai_tool_calls (
			id         TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			tool       TEXT NOT NULL DEFAULT '',
			action     TEXT NOT NULL DEFAULT '',
			resource   TEXT NOT NULL DEFAULT '',
			decision   TEXT NOT NULL DEFAULT '',
			reason     TEXT NOT NULL DEFAULT '',
			called_at  TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	for _, stmt := range initSecureSchemaStmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// ---- agents -----------------------------------------------------------------

func (s *Store) InsertAgent(a Agent) error {
	_, err := s.db.Exec(
		`INSERT INTO ai_agents (id, name, project, model, owner, role_template, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Project, a.Model, a.Owner, a.RoleTemplate, a.Status, a.CreatedAt,
	)
	return err
}

func (s *Store) GetAgent(nameOrID, project string) (Agent, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, model, owner, role_template, status, created_at
			 FROM ai_agents WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, model, owner, role_template, status, created_at
			 FROM ai_agents WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanAgent(row)
}

func (s *Store) ListAgents(project string) ([]Agent, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, model, owner, role_template, status, created_at FROM ai_agents ORDER BY name`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, model, owner, role_template, status, created_at FROM ai_agents WHERE project=? ORDER BY name`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAgentStatus(id string, status AgentStatus) error {
	_, err := s.db.Exec(`UPDATE ai_agents SET status=? WHERE id=?`, status, id)
	return err
}

// ---- sessions ---------------------------------------------------------------

func (s *Store) InsertSession(sess Session) error {
	_, err := s.db.Exec(
		`INSERT INTO ai_sessions (id, agent_id, project, principal, model, status, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.AgentID, sess.Project, sess.Principal, sess.Model, sess.Status, sess.StartedAt, sess.EndedAt,
	)
	return err
}

func (s *Store) GetSession(id string) (Session, error) {
	row := s.db.QueryRow(
		`SELECT id, agent_id, project, principal, model, status, started_at, ended_at
		 FROM ai_sessions WHERE id=? LIMIT 1`,
		id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions(project string) ([]Session, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, agent_id, project, principal, model, status, started_at, ended_at FROM ai_sessions ORDER BY started_at DESC`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, agent_id, project, principal, model, status, started_at, ended_at FROM ai_sessions WHERE project=? ORDER BY started_at DESC`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Store) EndSession(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE ai_sessions SET status=?, ended_at=? WHERE id=?`, SessionEnded, now, id)
	return err
}

// ---- mcp servers ------------------------------------------------------------

func (s *Store) InsertMCP(m MCPServer) error {
	_, err := s.db.Exec(
		`INSERT INTO ai_mcp_servers (id, name, project, endpoint, tools_json, iam_action, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Project, m.Endpoint, m.ToolsJSON, m.IAMAction, m.CreatedAt,
	)
	return err
}

func (s *Store) ListMCP(project string) ([]MCPServer, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, endpoint, tools_json, iam_action, created_at FROM ai_mcp_servers ORDER BY name`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, endpoint, tools_json, iam_action, created_at FROM ai_mcp_servers WHERE project=? ORDER BY name`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPServer
	for rows.Next() {
		m, err := scanMCP(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) DeleteMCP(nameOrID, project string) error {
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(`DELETE FROM ai_mcp_servers WHERE id=? OR name=?`, nameOrID, nameOrID)
	} else {
		res, err = s.db.Exec(`DELETE FROM ai_mcp_servers WHERE (id=? OR name=?) AND project=?`, nameOrID, nameOrID, project)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("mcp server %q not found", nameOrID)
	}
	return nil
}

// ---- tool calls -------------------------------------------------------------

func (s *Store) InsertToolCall(tc ToolCall) error {
	_, err := s.db.Exec(
		`INSERT INTO ai_tool_calls (id, session_id, tool, action, resource, decision, reason, called_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tc.ID, tc.SessionID, tc.Tool, tc.Action, tc.Resource, tc.Decision, tc.Reason, tc.CalledAt,
	)
	return err
}

func (s *Store) ListToolCalls(sessionID string) ([]ToolCall, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, tool, action, resource, decision, reason, called_at
		 FROM ai_tool_calls WHERE session_id=? ORDER BY called_at`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ToolCall
	for rows.Next() {
		var tc ToolCall
		if err := rows.Scan(&tc.ID, &tc.SessionID, &tc.Tool, &tc.Action, &tc.Resource, &tc.Decision, &tc.Reason, &tc.CalledAt); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// ---- scanners ---------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgent(s rowScanner) (Agent, error) {
	var a Agent
	if err := s.Scan(&a.ID, &a.Name, &a.Project, &a.Model, &a.Owner, &a.RoleTemplate, &a.Status, &a.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Agent{}, fmt.Errorf("ai agent not found")
		}
		return Agent{}, fmt.Errorf("ai agent: scan: %w", err)
	}
	return a, nil
}

func scanSession(s rowScanner) (Session, error) {
	var sess Session
	if err := s.Scan(&sess.ID, &sess.AgentID, &sess.Project, &sess.Principal, &sess.Model, &sess.Status, &sess.StartedAt, &sess.EndedAt); err != nil {
		if err == sql.ErrNoRows {
			return Session{}, fmt.Errorf("ai session not found")
		}
		return Session{}, fmt.Errorf("ai session: scan: %w", err)
	}
	return sess, nil
}

func scanMCP(s rowScanner) (MCPServer, error) {
	var m MCPServer
	if err := s.Scan(&m.ID, &m.Name, &m.Project, &m.Endpoint, &m.ToolsJSON, &m.IAMAction, &m.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return MCPServer{}, fmt.Errorf("mcp server not found")
		}
		return MCPServer{}, fmt.Errorf("mcp server: scan: %w", err)
	}
	return m, nil
}

func newID(prefix string) string {
	return prefix + "_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
