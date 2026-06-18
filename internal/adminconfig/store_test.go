package adminconfig

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := NewStore(db)
	if err := s.InitSchema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

func TestSetGetIntAndDelete(t *testing.T) {
	s := newStore(t)

	if _, ok, _ := s.GetInt(KeyHostDeploymentsMax); ok {
		t.Fatal("expected unset key")
	}
	if err := s.SetInt(KeyHostDeploymentsMax, 42); err != nil {
		t.Fatalf("set: %v", err)
	}
	n, ok, err := s.GetInt(KeyHostDeploymentsMax)
	if err != nil || !ok || n != 42 {
		t.Fatalf("got %d ok=%v err=%v, want 42", n, ok, err)
	}

	// Upsert replaces.
	if err := s.SetInt(KeyHostDeploymentsMax, 7); err != nil {
		t.Fatalf("set2: %v", err)
	}
	if n, _, _ := s.GetInt(KeyHostDeploymentsMax); n != 7 {
		t.Fatalf("got %d, want 7", n)
	}

	if err := s.Delete(KeyHostDeploymentsMax); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := s.GetInt(KeyHostDeploymentsMax); ok {
		t.Fatal("expected unset after delete")
	}
}

func TestGetIntUnparseable(t *testing.T) {
	s := newStore(t)
	if err := s.Set(KeyHostDeploymentsMax, "not-a-number"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// A set-but-unparseable value is treated as unset so callers fall back.
	if _, ok, _ := s.GetInt(KeyHostDeploymentsMax); ok {
		t.Fatal("expected unparseable value to read as unset")
	}
}
