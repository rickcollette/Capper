// Package audit stores structured audit events for every significant action
// taken by any principal. Events are append-only and must never be deleted.
//
// Every event records: org, account, project, actor identity, action taken,
// target resource CRN, decision (allow/deny), source IP, and arbitrary metadata.
package audit

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Event is a single immutable audit record.
type Event struct {
	ID           string `json:"id"`
	OrgID        string `json:"orgId"`
	AccountID    string `json:"accountId"`
	ProjectID    string `json:"projectId"`
	ActorType    string `json:"actorType"`
	ActorID      string `json:"actorId"`
	ActorURN     string `json:"actorUrn"`
	Action       string `json:"action"`
	ResourceCRN  string `json:"resourceCrn"`
	Decision     string `json:"decision"` // "allow" | "deny"
	SourceIP     string `json:"sourceIp"`
	UserAgent    string `json:"userAgent"`
	RequestID    string `json:"requestId"`
	MetadataJSON string `json:"metadata,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

// Store persists audit events.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by an already-initialised database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema creates the audit_events table. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS audit_events (
		id            TEXT PRIMARY KEY,
		org_id        TEXT NOT NULL,
		account_id    TEXT NOT NULL DEFAULT '',
		project_id    TEXT NOT NULL DEFAULT '',
		actor_type    TEXT NOT NULL,
		actor_id      TEXT NOT NULL,
		actor_urn     TEXT NOT NULL DEFAULT '',
		action        TEXT NOT NULL,
		resource_crn  TEXT NOT NULL DEFAULT '',
		decision      TEXT NOT NULL,
		source_ip     TEXT NOT NULL DEFAULT '',
		user_agent    TEXT NOT NULL DEFAULT '',
		request_id    TEXT NOT NULL DEFAULT '',
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at    TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("audit: schema: %w", err)
	}
	return nil
}

// Record appends a new audit event. The event ID must be set by the caller
// (use NewID for a collision-resistant value).
func (s *Store) Record(e Event) error {
	if e.CreatedAt == "" {
		e.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if e.MetadataJSON == "" {
		e.MetadataJSON = "{}"
	}
	_, err := s.db.Exec(
		`INSERT INTO audit_events
		 (id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		  action, resource_crn, decision, source_ip, user_agent, request_id,
		  metadata_json, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.OrgID, e.AccountID, e.ProjectID,
		e.ActorType, e.ActorID, e.ActorURN,
		e.Action, e.ResourceCRN, e.Decision,
		e.SourceIP, e.UserAgent, e.RequestID,
		e.MetadataJSON, e.CreatedAt,
	)
	return err
}

// ListByAccount returns events for an account in reverse chronological order.
// limit caps the result set; 0 means no cap (maximum 1000).
func (s *Store) ListByAccount(orgID, accountID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.Query(
		`SELECT id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		        action, resource_crn, decision, source_ip, user_agent, request_id,
		        metadata_json, created_at
		 FROM audit_events
		 WHERE org_id=? AND account_id=?
		 ORDER BY created_at DESC LIMIT ?`,
		orgID, accountID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// ListByOrg returns all events for an organization, most-recent first.
func (s *Store) ListByOrg(orgID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.Query(
		`SELECT id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		        action, resource_crn, decision, source_ip, user_agent, request_id,
		        metadata_json, created_at
		 FROM audit_events WHERE org_id=?
		 ORDER BY created_at DESC LIMIT ?`,
		orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// ListDenials returns events where decision == "deny" for an account.
func (s *Store) ListDenials(orgID, accountID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.Query(
		`SELECT id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		        action, resource_crn, decision, source_ip, user_agent, request_id,
		        metadata_json, created_at
		 FROM audit_events
		 WHERE org_id=? AND account_id=? AND decision='deny'
		 ORDER BY created_at DESC LIMIT ?`,
		orgID, accountID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// WithMetadata attaches arbitrary key/value metadata to an event.
func WithMetadata(e *Event, kv map[string]any) error {
	b, err := json.Marshal(kv)
	if err != nil {
		return err
	}
	e.MetadataJSON = string(b)
	return nil
}

// NewID generates a time-ordered audit event ID with a random suffix to avoid
// collisions on fast bursts of events.
func NewID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("evt_%d_%x", time.Now().UnixNano(), b)
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.AccountID, &e.ProjectID,
			&e.ActorType, &e.ActorID, &e.ActorURN,
			&e.Action, &e.ResourceCRN, &e.Decision,
			&e.SourceIP, &e.UserAgent, &e.RequestID,
			&e.MetadataJSON, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
