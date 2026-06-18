// Package observability implements Phase 2 (unified logs) and Phase 4
// (alert rules and JSONL export) of the Observability spec.
//
// Phase 1 (event rules) and Phase 3 (Prometheus metrics) live in
// internal/eventing/ and internal/metrics/ respectively.
package observability

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Log levels.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Alert states.
const (
	AlertStateFiring   = "firing"
	AlertStateResolved = "resolved"
)

// LogEntry is a structured log record attached to a named resource.
type LogEntry struct {
	ID         string            `json:"id"`
	Resource   string            `json:"resource"` // e.g., "instance/web01"
	Service    string            `json:"service"`  // subsystem name
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Labels     map[string]string `json:"labels,omitempty"`
	LabelsJSON string            `json:"-"`
	Timestamp  string            `json:"timestamp"`
}

// AlertRule defines when an alert should fire.
type AlertRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Condition   string `json:"condition"` // e.g., "error_rate > 0.05"
	Severity    string `json:"severity"`  // "critical","high","medium","low"
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
}

// AlertEvent is a fired or resolved alert instance.
type AlertEvent struct {
	ID        string `json:"id"`
	RuleID    string `json:"ruleId"`
	RuleName  string `json:"ruleName"`
	State     string `json:"state"`
	Message   string `json:"message"`
	FiredAt   string `json:"firedAt"`
	ResolvedAt string `json:"resolvedAt,omitempty"`
}

// Store persists logs and alerts.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the unified log and alert tables.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS obs_logs (
			id        TEXT PRIMARY KEY,
			resource  TEXT NOT NULL,
			service   TEXT NOT NULL DEFAULT '',
			level     TEXT NOT NULL DEFAULT 'info',
			message   TEXT NOT NULL,
			labels    TEXT NOT NULL DEFAULT '{}',
			timestamp TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS obs_logs_resource ON obs_logs(resource)`,
		`CREATE INDEX IF NOT EXISTS obs_logs_service  ON obs_logs(service)`,
		`CREATE TABLE IF NOT EXISTS obs_alert_rules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			condition   TEXT NOT NULL,
			severity    TEXT NOT NULL DEFAULT 'medium',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS obs_alert_events (
			id          TEXT PRIMARY KEY,
			rule_id     TEXT NOT NULL,
			rule_name   TEXT NOT NULL,
			state       TEXT NOT NULL DEFAULT 'firing',
			message     TEXT NOT NULL DEFAULT '',
			fired_at    TEXT NOT NULL,
			resolved_at TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("observability: schema: %w", err)
		}
	}
	return nil
}

// ---- logs -------------------------------------------------------------------

func (s *Store) AppendLog(e LogEntry) error {
	if e.ID == "" {
		e.ID = fmt.Sprintf("log_%d", time.Now().UnixNano())
	}
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	labels := e.LabelsJSON
	if labels == "" {
		b, _ := json.Marshal(e.Labels)
		labels = string(b)
	}
	if labels == "" {
		labels = "{}"
	}
	_, err := s.db.Exec(
		`INSERT INTO obs_logs (id, resource, service, level, message, labels, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Resource, e.Service, e.Level, e.Message, labels, e.Timestamp,
	)
	return err
}

// LogsByResource returns log entries for a specific resource, newest first.
func (s *Store) LogsByResource(resource string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, resource, service, level, message, labels, timestamp
		 FROM obs_logs WHERE resource=? ORDER BY timestamp DESC LIMIT ?`,
		resource, limit,
	)
	if err != nil {
		return nil, err
	}
	return scanLogs(rows)
}

// LogsByLabel returns log entries that contain the given label key/value pair.
func (s *Store) LogsByLabel(key, value string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	// SQLite JSON path search via LIKE (sufficient for MVP).
	pattern := fmt.Sprintf(`%%"%s":"%s"%%`, key, value)
	rows, err := s.db.Query(
		`SELECT id, resource, service, level, message, labels, timestamp
		 FROM obs_logs WHERE labels LIKE ? ORDER BY timestamp DESC LIMIT ?`,
		pattern, limit,
	)
	if err != nil {
		return nil, err
	}
	return scanLogs(rows)
}

// LogsByService returns combined log entries for a named service.
func (s *Store) LogsByService(service string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, resource, service, level, message, labels, timestamp
		 FROM obs_logs WHERE service=? ORDER BY timestamp DESC LIMIT ?`,
		service, limit,
	)
	if err != nil {
		return nil, err
	}
	return scanLogs(rows)
}

func scanLogs(rows *sql.Rows) ([]LogEntry, error) {
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Resource, &e.Service, &e.Level, &e.Message, &e.LabelsJSON, &e.Timestamp); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(e.LabelsJSON), &e.Labels)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExportJSONL writes all log entries for resource (or all if empty) as JSONL
// to a strings.Builder and returns the result. Each line is a JSON object.
func (s *Store) ExportJSONL(resource string) (string, error) {
	var rows *sql.Rows
	var err error
	if resource != "" {
		rows, err = s.db.Query(
			`SELECT id, resource, service, level, message, labels, timestamp FROM obs_logs WHERE resource=? ORDER BY timestamp`,
			resource,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, resource, service, level, message, labels, timestamp FROM obs_logs ORDER BY timestamp`,
		)
	}
	if err != nil {
		return "", err
	}
	entries, err := scanLogs(rows)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			continue
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// ---- alert rules ------------------------------------------------------------

func (s *Store) CreateAlertRule(r AlertRule) (AlertRule, error) {
	if r.ID == "" {
		r.ID = fmt.Sprintf("ar_%d", time.Now().UnixNano())
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	r.Enabled = true
	_, err := s.db.Exec(
		`INSERT INTO obs_alert_rules (id, name, description, condition, severity, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Description, r.Condition, r.Severity, 1, r.CreatedAt,
	)
	return r, err
}

func (s *Store) ListAlertRules() ([]AlertRule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, condition, severity, enabled, created_at FROM obs_alert_rules ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		var r AlertRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Condition, &r.Severity, &enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) EnableAlertRule(id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.Exec(`UPDATE obs_alert_rules SET enabled=? WHERE id=?`, v, id)
	return err
}

func (s *Store) DeleteAlertRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM obs_alert_rules WHERE id=?`, id)
	return err
}

// ---- alert events -----------------------------------------------------------

func (s *Store) FireAlert(ruleID, ruleName, message string) (AlertEvent, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	e := AlertEvent{
		ID:       fmt.Sprintf("ae_%d", time.Now().UnixNano()),
		RuleID:   ruleID,
		RuleName: ruleName,
		State:    AlertStateFiring,
		Message:  message,
		FiredAt:  now,
	}
	_, err := s.db.Exec(
		`INSERT INTO obs_alert_events (id, rule_id, rule_name, state, message, fired_at, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.RuleID, e.RuleName, e.State, e.Message, e.FiredAt, "",
	)
	return e, err
}

func (s *Store) ResolveAlert(eventID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE obs_alert_events SET state=?, resolved_at=? WHERE id=?`,
		AlertStateResolved, now, eventID,
	)
	return err
}

func (s *Store) ListAlertEvents(state string) ([]AlertEvent, error) {
	var rows *sql.Rows
	var err error
	if state != "" {
		rows, err = s.db.Query(
			`SELECT id, rule_id, rule_name, state, message, fired_at, resolved_at FROM obs_alert_events WHERE state=? ORDER BY fired_at DESC`,
			state,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, rule_id, rule_name, state, message, fired_at, resolved_at FROM obs_alert_events ORDER BY fired_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.RuleID, &e.RuleName, &e.State, &e.Message, &e.FiredAt, &e.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
