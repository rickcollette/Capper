package audit

import (
	"fmt"
	"net/http"
	"time"
)

// Recorder is a helper attached to each Server to emit audit events.
type Recorder struct {
	store *Store
}

// NewRecorder creates a Recorder backed by the given Store.
func NewRecorder(store *Store) *Recorder { return &Recorder{store: store} }

// EventParams holds the caller-supplied fields for a single audit event.
type EventParams struct {
	OrgID       string
	AccountID   string
	ProjectID   string
	ActorType   string
	ActorID     string
	Action      string
	ResourceCRN string
	Decision    string
	Metadata    string
}

// Record emits an audit event derived from the HTTP request and EventParams.
// Errors are silently dropped to avoid disrupting the caller.
func (rec *Recorder) Record(r *http.Request, p EventParams) {
	if p.Decision == "" {
		p.Decision = "success"
	}
	e := AuditEvent{
		OrgID:        p.OrgID,
		AccountID:    p.AccountID,
		ProjectID:    p.ProjectID,
		ActorType:    p.ActorType,
		ActorID:      p.ActorID,
		ActorURN:     fmt.Sprintf("crn:capper:iam:::%s/%s", p.ActorType, p.ActorID),
		Action:       p.Action,
		ResourceCRN:  p.ResourceCRN,
		Decision:     p.Decision,
		SourceIP:     r.RemoteAddr,
		UserAgent:    r.Header.Get("User-Agent"),
		RequestID:    r.Header.Get("X-Request-ID"),
		MetadataJSON: p.Metadata,
		CreatedAt:    time.Now().UTC(),
	}
	_ = rec.store.InsertEvent(e)
}
