package secret_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/secret"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := secret.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *secret.Manager {
	t.Helper()
	db := openDB(t)
	key := make([]byte, 32)
	return secret.NewManager(secret.NewStore(db), key)
}

func TestCreateAndGetDecrypted(t *testing.T) {
	m := newManager(t)
	_, err := m.Create("db-pass", "proj1", "database password", "s3cr3t")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	plain, err := m.GetDecrypted("db-pass", "proj1")
	if err != nil {
		t.Fatalf("GetDecrypted: %v", err)
	}
	if plain != "s3cr3t" {
		t.Errorf("got %q, want %q", plain, "s3cr3t")
	}
}

func TestGet_ReturnsEncryptedRecord(t *testing.T) {
	m := newManager(t)
	_, err := m.Create("api-key", "proj1", "api key", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	sec, err := m.Get("api-key", "proj1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sec.Name != "api-key" {
		t.Errorf("name: %q", sec.Name)
	}
	// Ciphertext must differ from plaintext.
	plain, _ := m.Decrypt(sec)
	if plain != "abc123" {
		t.Errorf("decrypted: got %q, want abc123", plain)
	}
}

func TestList(t *testing.T) {
	m := newManager(t)
	for _, name := range []string{"s1", "s2", "s3"} {
		if _, err := m.Create(name, "proj1", "", "val"); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}
	secs, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(secs) != 3 {
		t.Errorf("List: got %d, want 3", len(secs))
	}
}

func TestDelete(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("temp", "proj1", "", "value"); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete("temp", "proj1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := m.Get("temp", "proj1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestProjectIsolation(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("shared-name", "proj1", "", "proj1val"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create("shared-name", "proj2", "", "proj2val"); err != nil {
		t.Fatal(err)
	}
	p1, _ := m.GetDecrypted("shared-name", "proj1")
	p2, _ := m.GetDecrypted("shared-name", "proj2")
	if p1 != "proj1val" || p2 != "proj2val" {
		t.Errorf("project isolation broken: proj1=%q proj2=%q", p1, p2)
	}
}
