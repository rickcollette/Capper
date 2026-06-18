package backup_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"capper/internal/backup"

	_ "modernc.org/sqlite"
)

func newManager(t *testing.T) *backup.Manager {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := backup.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return backup.NewManager(backup.NewStore(db), db, t.TempDir())
}

func TestBackupDatabaseRunsPgDumpAndRecordsArtifact(t *testing.T) {
	mgr := newManager(t)
	var gotName string
	var gotArgs []string
	mgr.SetCommandRunner(func(_ context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = append([]string{}, args...)
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--file" {
				return os.WriteFile(args[i+1], []byte("real dump bytes"), 0o600)
			}
		}
		t.Fatalf("pg_dump missing --file arg: %v", args)
		return nil
	})

	rec, err := mgr.BackupDatabase("default", "app/db", "postgres://user:pass@127.0.0.1/app", "")
	if err != nil {
		t.Fatalf("BackupDatabase: %v", err)
	}
	if gotName != "pg_dump" {
		t.Fatalf("expected pg_dump, got %q", gotName)
	}
	if len(gotArgs) != 4 || gotArgs[0] != "--format=custom" || gotArgs[1] != "--file" {
		t.Fatalf("unexpected pg_dump args: %v", gotArgs)
	}
	if gotArgs[3] != "postgres://user:pass@127.0.0.1/app" {
		t.Fatalf("connection string not passed to pg_dump: %v", gotArgs)
	}
	if rec.Type != backup.BackupTypeDatabase {
		t.Fatalf("expected database backup, got %q", rec.Type)
	}
	if rec.SizeBytes != int64(len("real dump bytes")) {
		t.Fatalf("expected real size, got %d", rec.SizeBytes)
	}
	if filepath.Base(rec.Path)[:6] != "app_db" {
		t.Fatalf("backup filename was not sanitized: %s", rec.Path)
	}
}

func TestRestoreDatabaseRunsPgRestore(t *testing.T) {
	mgr := newManager(t)
	mgr.SetCommandRunner(func(_ context.Context, _ string, args ...string) error {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--file" {
				return os.WriteFile(args[i+1], []byte("dump"), 0o600)
			}
		}
		return nil
	})
	rec, err := mgr.BackupDatabase("default", "app", "postgres://source/app", "")
	if err != nil {
		t.Fatalf("BackupDatabase: %v", err)
	}

	var gotName string
	var gotArgs []string
	mgr.SetCommandRunner(func(_ context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = append([]string{}, args...)
		return nil
	})
	if err := mgr.RestoreDatabase(rec.ID, "default", "postgres://target/app_restored"); err != nil {
		t.Fatalf("RestoreDatabase: %v", err)
	}
	if gotName != "pg_restore" {
		t.Fatalf("expected pg_restore, got %q", gotName)
	}
	want := []string{"--clean", "--if-exists", "--dbname", "postgres://target/app_restored", rec.Path}
	if len(gotArgs) != len(want) {
		t.Fatalf("unexpected pg_restore args: %v", gotArgs)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Fatalf("arg %d: got %q want %q; all args %v", i, gotArgs[i], want[i], gotArgs)
		}
	}
}

func TestBackupDatabaseRequiresConnectionString(t *testing.T) {
	mgr := newManager(t)
	if _, err := mgr.BackupDatabase("default", "app", "", ""); err == nil {
		t.Fatal("expected missing connection string error")
	}
}

func TestRunDuePoliciesRunsDatabaseBackups(t *testing.T) {
	mgr := newManager(t)
	var pgDumpRuns int
	mgr.SetCommandRunner(func(_ context.Context, name string, args ...string) error {
		if name != "pg_dump" {
			t.Fatalf("unexpected command %q", name)
		}
		pgDumpRuns++
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--file" {
				return os.WriteFile(args[i+1], []byte("scheduled dump"), 0o600)
			}
		}
		t.Fatalf("pg_dump missing --file arg: %v", args)
		return nil
	})
	if _, err := mgr.CreatePolicyWithSource("appdb", "default", "", "postgres://source/app", backup.BackupTypeDatabase, 1, 5); err != nil {
		t.Fatalf("CreatePolicyWithSource: %v", err)
	}
	if err := mgr.RunDuePolicies("default"); err != nil {
		t.Fatalf("RunDuePolicies: %v", err)
	}
	if pgDumpRuns != 1 {
		t.Fatalf("expected one pg_dump run, got %d", pgDumpRuns)
	}
	records, err := mgr.ListDatabaseBackups("default")
	if err != nil {
		t.Fatalf("ListDatabaseBackups: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one database backup record, got %d", len(records))
	}
	policies, err := mgr.ListPolicies("default")
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	if len(policies) != 1 || policies[0].LastRunAt == "" {
		t.Fatalf("policy last run not updated: %+v", policies)
	}
}
