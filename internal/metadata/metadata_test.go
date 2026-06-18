package metadata_test

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/metadata"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *metadata.Manager {
	st := metadata.NewStore(openDB(t))
	return metadata.NewManager(st, nil)
}

// ---- Store ------------------------------------------------------------------

func TestStoreUpsertAndGet(t *testing.T) {
	st := metadata.NewStore(openDB(t))
	meta := metadata.InstanceMetadata{
		InstanceID:   "inst-1",
		Hostname:     "web01",
		Project:      "proj1",
		InstanceType: "t2.micro",
		NetworkIP:    "10.0.1.5",
		UserData:     "#!/bin/sh\necho hello",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := st.Upsert(meta); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := st.Get("inst-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Hostname != "web01" {
		t.Errorf("hostname: %q", got.Hostname)
	}
	if got.NetworkIP != "10.0.1.5" {
		t.Errorf("network ip: %q", got.NetworkIP)
	}
}

func TestStoreUpsert_Idempotent(t *testing.T) {
	st := metadata.NewStore(openDB(t))
	meta := metadata.InstanceMetadata{
		InstanceID: "inst-1",
		Hostname:   "web01",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := st.Upsert(meta); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	meta.Hostname = "web01-updated"
	if err := st.Upsert(meta); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, _ := st.Get("inst-1")
	if got.Hostname != "web01-updated" {
		t.Errorf("hostname after update: %q", got.Hostname)
	}
}

func TestStoreGet_NotFound(t *testing.T) {
	st := metadata.NewStore(openDB(t))
	_, err := st.Get("no-such-instance")
	if err == nil {
		t.Error("expected error for missing instance")
	}
}

// ---- TokenManager -----------------------------------------------------------

func TestTokenIssueAndValidate(t *testing.T) {
	tm := metadata.NewTokenManager()
	token, err := tm.Issue("inst-1", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	if !tm.Validate(token, "inst-1") {
		t.Error("expected token to be valid")
	}
}

func TestToken_WrongInstance(t *testing.T) {
	tm := metadata.NewTokenManager()
	token, _ := tm.Issue("inst-1", time.Hour)
	if tm.Validate(token, "inst-2") {
		t.Error("token should not validate for different instance")
	}
}

func TestToken_Expired(t *testing.T) {
	tm := metadata.NewTokenManager()
	// Issue with negative TTL (already expired).
	token, _ := tm.Issue("inst-1", -time.Second)
	if tm.Validate(token, "inst-1") {
		t.Error("expired token should not validate")
	}
}

// ---- Manager ----------------------------------------------------------------

func TestManagerCreateRecord(t *testing.T) {
	m := newManager(t)
	meta := metadata.InstanceMetadata{
		InstanceID: "inst-2",
		Hostname:   "api01",
		Project:    "proj1",
		NetworkIP:  "10.0.2.5",
	}
	token, err := m.CreateRecord(meta)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if token == "" {
		t.Error("expected launch token")
	}

	got, err := m.GetRecord("inst-2")
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if got.Hostname != "api01" {
		t.Errorf("hostname: %q", got.Hostname)
	}
	// Token hash should be persisted.
	if got.TokenHash == "" {
		t.Error("token hash should be persisted in record")
	}
}

func TestManagerDeleteRecord(t *testing.T) {
	m := newManager(t)
	m.CreateRecord(metadata.InstanceMetadata{
		InstanceID: "inst-3",
		Hostname:   "del-host",
	})
	if err := m.DeleteRecord("inst-3"); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	_, err := m.GetRecord("inst-3")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestManagerLookupByIP(t *testing.T) {
	m := newManager(t)
	m.CreateRecord(metadata.InstanceMetadata{
		InstanceID: "inst-4",
		Hostname:   "db01",
		NetworkIP:  "10.5.5.5",
	})
	rec, ok := m.LookupByIP("10.5.5.5")
	if !ok {
		t.Fatal("expected LookupByIP to find record")
	}
	if rec.InstanceID != "inst-4" {
		t.Errorf("instanceID: %q", rec.InstanceID)
	}
}
