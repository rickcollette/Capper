package audit_test

import (
	"database/sql"
	"testing"

	"capper/internal/audit"

	_ "modernc.org/sqlite"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := audit.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

func TestRecordAndList(t *testing.T) {
	db := openDB(t)
	s := audit.NewStore(db)

	e := audit.Event{
		ID:        audit.NewID(),
		OrgID:     "org_1",
		AccountID: "acct_1",
		ActorType: "iam-user",
		ActorID:   "usr_1",
		Action:    "compute:instance:create",
		Decision:  "allow",
	}
	if err := s.Record(e); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, err := s.ListByAccount("org_1", "acct_1", 10)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Action != "compute:instance:create" {
		t.Errorf("action = %q want compute:instance:create", events[0].Action)
	}
}

func TestListDenials(t *testing.T) {
	db := openDB(t)
	s := audit.NewStore(db)

	_ = s.Record(audit.Event{
		ID: audit.NewID(), OrgID: "org_1", AccountID: "acct_1",
		ActorType: "iam-user", ActorID: "usr_1",
		Action: "vpc:delete", Decision: "allow",
	})
	_ = s.Record(audit.Event{
		ID: audit.NewID(), OrgID: "org_1", AccountID: "acct_1",
		ActorType: "iam-user", ActorID: "usr_1",
		Action: "instance:terminate", Decision: "deny",
	})

	denials, err := s.ListDenials("org_1", "acct_1", 10)
	if err != nil {
		t.Fatalf("ListDenials: %v", err)
	}
	if len(denials) != 1 {
		t.Fatalf("expected 1 denial, got %d", len(denials))
	}
	if denials[0].Decision != "deny" {
		t.Errorf("decision = %q want deny", denials[0].Decision)
	}
}

func TestAccountIsolation(t *testing.T) {
	db := openDB(t)
	s := audit.NewStore(db)

	_ = s.Record(audit.Event{
		ID: audit.NewID(), OrgID: "org_1", AccountID: "acct_1",
		ActorType: "iam-user", ActorID: "u1",
		Action: "compute:list", Decision: "allow",
	})
	_ = s.Record(audit.Event{
		ID: audit.NewID(), OrgID: "org_1", AccountID: "acct_2",
		ActorType: "iam-user", ActorID: "u2",
		Action: "compute:list", Decision: "allow",
	})

	events, err := s.ListByAccount("org_1", "acct_1", 10)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event for acct_1, got %d", len(events))
	}
}

func TestOrgListSeesAllAccounts(t *testing.T) {
	db := openDB(t)
	s := audit.NewStore(db)

	for _, acct := range []string{"acct_1", "acct_2", "acct_3"} {
		_ = s.Record(audit.Event{
			ID: audit.NewID(), OrgID: "org_1", AccountID: acct,
			ActorType: "iam-user", ActorID: "u",
			Action: "compute:list", Decision: "allow",
		})
	}

	events, err := s.ListByOrg("org_1", 10)
	if err != nil {
		t.Fatalf("ListByOrg: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestMetadataAttachment(t *testing.T) {
	db := openDB(t)
	s := audit.NewStore(db)

	e := audit.Event{
		ID:        audit.NewID(),
		OrgID:     "org_1",
		AccountID: "acct_1",
		ActorType: "system",
		ActorID:   "scheduler",
		Action:    "instance:place",
		Decision:  "allow",
	}
	if err := audit.WithMetadata(&e, map[string]any{"node": "node-7", "zone": "use1-a"}); err != nil {
		t.Fatalf("WithMetadata: %v", err)
	}
	if err := s.Record(e); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, _ := s.ListByAccount("org_1", "acct_1", 1)
	if len(events) == 0 {
		t.Fatal("no events returned")
	}
	if events[0].MetadataJSON == "{}" || events[0].MetadataJSON == "" {
		t.Error("expected non-empty metadata")
	}
}
