package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventFilter holds optional filter parameters for ListEvents.
type EventFilter struct {
	OrgID     string
	AccountID string
	ProjectID string
	ActorID   string
	Action    string
	Decision  string
	Before    string
	Limit     int
}

// InsertEvent persists an AuditEvent. An ID and timestamp are generated if absent.
func (s *Store) InsertEvent(e AuditEvent) error {
	if e.ID == "" {
		e.ID = "evt_" + uuid.New().String()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	metadata := e.MetadataJSON
	if metadata == "" {
		metadata = "{}"
	}
	_, err := s.db.Exec(`INSERT INTO audit_events
		(id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		 action, resource_crn, decision, source_ip, user_agent, request_id, metadata_json, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.OrgID, e.AccountID, e.ProjectID, e.ActorType, e.ActorID, e.ActorURN,
		e.Action, e.ResourceCRN, e.Decision, e.SourceIP, e.UserAgent, e.RequestID,
		metadata, e.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

// ListEvents returns audit events matching the filter, most-recent first.
func (s *Store) ListEvents(f EventFilter) ([]AuditEvent, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	conditions := []string{}
	args := []any{}
	if f.OrgID != "" {
		conditions = append(conditions, "org_id=?")
		args = append(args, f.OrgID)
	}
	if f.AccountID != "" {
		conditions = append(conditions, "account_id=?")
		args = append(args, f.AccountID)
	}
	if f.ProjectID != "" {
		conditions = append(conditions, "project_id=?")
		args = append(args, f.ProjectID)
	}
	if f.ActorID != "" {
		conditions = append(conditions, "actor_id=?")
		args = append(args, f.ActorID)
	}
	if f.Action != "" {
		conditions = append(conditions, "action=?")
		args = append(args, f.Action)
	}
	if f.Decision != "" {
		conditions = append(conditions, "decision=?")
		args = append(args, f.Decision)
	}
	if f.Before != "" {
		conditions = append(conditions, "created_at<?")
		args = append(args, f.Before)
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	args = append(args, f.Limit)
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		        action, resource_crn, decision, source_ip, user_agent, request_id, metadata_json, created_at
		 FROM audit_events %s ORDER BY created_at DESC LIMIT ?`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.OrgID, &e.AccountID, &e.ProjectID,
			&e.ActorType, &e.ActorID, &e.ActorURN, &e.Action, &e.ResourceCRN,
			&e.Decision, &e.SourceIP, &e.UserAgent, &e.RequestID, &e.MetadataJSON, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// ListByResource returns audit events for a specific resource CRN, most-recent first.
func (s *Store) ListByResource(resourceCRN string, limit int) ([]AuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, org_id, account_id, project_id, actor_type, actor_id, actor_urn,
		        action, resource_crn, decision, source_ip, user_agent, request_id, metadata_json, created_at
		 FROM audit_events WHERE resource_crn=? ORDER BY created_at DESC LIMIT ?`, resourceCRN, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.OrgID, &e.AccountID, &e.ProjectID,
			&e.ActorType, &e.ActorID, &e.ActorURN, &e.Action, &e.ResourceCRN,
			&e.Decision, &e.SourceIP, &e.UserAgent, &e.RequestID, &e.MetadataJSON, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}
