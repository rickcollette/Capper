package cert_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/cert"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := cert.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *cert.Manager {
	t.Helper()
	ca, err := cert.LoadOrCreateCA(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreateCA: %v", err)
	}
	return cert.NewManager(ca, cert.NewStore(openDB(t)))
}

func TestCAInit(t *testing.T) {
	dir := t.TempDir()
	ca, err := cert.LoadOrCreateCA(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateCA: %v", err)
	}
	m := cert.NewManager(ca, cert.NewStore(openDB(t)))
	pem := m.CACertPEM()
	if len(pem) == 0 {
		t.Error("CA cert PEM must not be empty")
	}
}

func TestCAIdempotent(t *testing.T) {
	dir := t.TempDir()
	ca1, err := cert.LoadOrCreateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	ca2, err := cert.LoadOrCreateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	m1 := cert.NewManager(ca1, cert.NewStore(openDB(t)))
	m2 := cert.NewManager(ca2, cert.NewStore(openDB(t)))
	if string(m1.CACertPEM()) != string(m2.CACertPEM()) {
		t.Error("CA cert changed between loads from same dir")
	}
}

func TestIssueAndGet(t *testing.T) {
	m := newManager(t)
	res, err := m.Issue("web-tls", "proj1", "web.local", []string{"web.local", "localhost"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if res.CertPEM == "" || res.KeyPEM == "" {
		t.Error("Issue must return non-empty cert and key PEM")
	}
	rec, err := m.Get("web-tls", "proj1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Name != "web-tls" {
		t.Errorf("name: got %q", rec.Name)
	}
}

func TestList(t *testing.T) {
	m := newManager(t)
	for _, name := range []string{"c1", "c2", "c3"} {
		if _, err := m.Issue(name, "proj1", name+".local", nil); err != nil {
			t.Fatalf("Issue %q: %v", name, err)
		}
	}
	certs, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(certs) != 3 {
		t.Errorf("List: got %d, want 3", len(certs))
	}
}

func TestRevoke(t *testing.T) {
	m := newManager(t)
	if _, err := m.Issue("revokeme", "proj1", "rev.local", nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Revoke("revokeme", "proj1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	rec, err := m.Get("revokeme", "proj1")
	if err != nil {
		t.Fatalf("Get after revoke: %v", err)
	}
	if rec.Status != "revoked" {
		t.Errorf("status after revoke: got %q, want revoked", rec.Status)
	}
}
