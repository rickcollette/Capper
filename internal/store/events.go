package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ResourceEvent records a lifecycle action on any Capper resource.
type ResourceEvent struct {
	ID            string         `json:"id"`
	ResourceType  string         `json:"resourceType"`
	ResourceID    string         `json:"resourceId"`
	Action        string         `json:"action"`
	ProjectID     string         `json:"projectId"`
	PrincipalType string         `json:"principalType"`
	PrincipalID   string         `json:"principalId"`
	Data          map[string]any `json:"data,omitempty"`
	Timestamp     string         `json:"timestamp"`
}

// EventStore persists resource lifecycle events to SQLite.
type EventStore struct {
	db *sql.DB
}

func newEventStore(db *sql.DB) *EventStore { return &EventStore{db: db} }

// InitSchema creates the resource_events table if it does not exist.
func InitEventSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS resource_events (
		id            TEXT    PRIMARY KEY,
		resource_type TEXT    NOT NULL,
		resource_id   TEXT    NOT NULL DEFAULT '',
		action        TEXT    NOT NULL,
		project_id    TEXT    NOT NULL DEFAULT '',
		principal_type TEXT   NOT NULL DEFAULT '',
		principal_id  TEXT    NOT NULL DEFAULT '',
		data          TEXT    NOT NULL DEFAULT '{}',
		timestamp     TEXT    NOT NULL
	)`)
	return err
}

// Insert persists e. The ID and Timestamp fields are set if empty.
func (s *EventStore) Insert(e ResourceEvent) error {
	if e.ID == "" {
		e.ID = newEventID()
	}
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(e.Data)
	if err != nil {
		data = []byte("{}")
	}
	_, err = s.db.Exec(
		`INSERT INTO resource_events
			(id, resource_type, resource_id, action, project_id, principal_type, principal_id, data, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ResourceType, e.ResourceID, e.Action,
		e.ProjectID, e.PrincipalType, e.PrincipalID, string(data), e.Timestamp,
	)
	return err
}

// ListOptions filters for EventStore.List.
type ListEventsOptions struct {
	ResourceType string
	ResourceID   string
	ProjectID    string
	Action       string
	Since        string
	Limit        int
}

// List returns events matching opts, newest first.
func (s *EventStore) List(opts ListEventsOptions) ([]ResourceEvent, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	q := `SELECT id, resource_type, resource_id, action, project_id,
	             principal_type, principal_id, data, timestamp
	      FROM resource_events WHERE 1=1`
	var args []any
	if opts.ResourceType != "" {
		q += " AND resource_type = ?"
		args = append(args, opts.ResourceType)
	}
	if opts.ResourceID != "" {
		q += " AND resource_id = ?"
		args = append(args, opts.ResourceID)
	}
	if opts.ProjectID != "" {
		q += " AND project_id = ?"
		args = append(args, opts.ProjectID)
	}
	if opts.Action != "" {
		q += " AND action LIKE ?"
		args = append(args, opts.Action+"%")
	}
	if opts.Since != "" {
		q += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}
	q += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT %d", opts.Limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ResourceEvent, 0)
	for rows.Next() {
		var e ResourceEvent
		var dataStr string
		if err := rows.Scan(&e.ID, &e.ResourceType, &e.ResourceID, &e.Action,
			&e.ProjectID, &e.PrincipalType, &e.PrincipalID, &dataStr, &e.Timestamp); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(dataStr), &e.Data)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Since returns all events with timestamp > cursor, oldest first. Used by tail.
func (s *EventStore) Since(cursor string, limit int) ([]ResourceEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT id, resource_type, resource_id, action, project_id,
		        principal_type, principal_id, data, timestamp
		 FROM resource_events WHERE timestamp > ?
		 ORDER BY timestamp ASC LIMIT ?`,
		cursor, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ResourceEvent, 0)
	for rows.Next() {
		var e ResourceEvent
		var dataStr string
		if err := rows.Scan(&e.ID, &e.ResourceType, &e.ResourceID, &e.Action,
			&e.ProjectID, &e.PrincipalType, &e.PrincipalID, &dataStr, &e.Timestamp); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(dataStr), &e.Data)
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestTimestamp returns the timestamp of the most recent event, or "" if none.
func (s *EventStore) LatestTimestamp() string {
	var ts string
	_ = s.db.QueryRow(`SELECT timestamp FROM resource_events ORDER BY timestamp DESC LIMIT 1`).Scan(&ts)
	return ts
}

func newEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
