//go:build integration

package backup_test

// Integration tests for the real pg_dump/pg_restore path.
//
// Prerequisites:
//   - pg_dump and pg_restore binaries must be on PATH
//   - CAPPER_TEST_POSTGRES_DSN must point to a live Postgres instance, e.g.:
//       postgres://postgres:postgres@localhost:5432/postgres
//
// Run with:
//   go test -tags integration ./internal/backup/...
//
// Or via docker:
//   docker compose -f docker-compose.yml up -d postgres
//   CAPPER_TEST_POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/postgres \
//     go test -tags integration ./internal/backup/...
//   docker compose -f docker-compose.yml down

import (
	"database/sql"
	"os"
	"os/exec"
	"testing"

	"capper/internal/backup"
	_ "modernc.org/sqlite"
)

func requirePostgres(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("CAPPER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CAPPER_TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skip("pg_dump not on PATH; skipping Postgres integration test")
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		t.Skip("pg_restore not on PATH; skipping Postgres integration test")
	}
	return dsn
}

func newIntegrationManager(t *testing.T) *backup.Manager {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := backup.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return backup.NewManager(backup.NewStore(db), db, t.TempDir())
}

func TestBackupDatabase_Real(t *testing.T) {
	dsn := requirePostgres(t)
	dir := t.TempDir()

	mgr := newIntegrationManager(t)

	rec, err := mgr.BackupDatabase("default", "integration-db", dsn, "")
	if err != nil {
		t.Fatalf("BackupDatabase: %v", err)
	}
	_ = dir
	if rec.ID == "" {
		t.Error("expected non-empty backup ID")
	}
	if rec.SizeBytes == 0 {
		t.Error("expected non-zero backup size")
	}
	if _, err := os.Stat(rec.Path); err != nil {
		t.Errorf("backup file missing: %v", err)
	}
	t.Logf("backup: id=%s size=%d path=%s", rec.ID, rec.SizeBytes, rec.Path)
}

func TestRestoreDatabase_Real(t *testing.T) {
	dsn := requirePostgres(t)
	dir := t.TempDir()

	mgr := newIntegrationManager(t)

	rec, err := mgr.BackupDatabase("default", "restore-test-db", dsn, "")
	if err != nil {
		t.Fatalf("BackupDatabase: %v", err)
	}
	_ = dir

	// Restore to the same DSN (idempotent for empty databases).
	if err := mgr.RestoreDatabase(rec.ID, "default", dsn); err != nil {
		t.Fatalf("RestoreDatabase: %v", err)
	}
}
