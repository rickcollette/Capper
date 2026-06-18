package database_test

import (
	"database/sql"
	"fmt"
	"testing"

	"capper/internal/database"

	_ "modernc.org/sqlite"
)

func newManager(t *testing.T) *database.Manager {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return database.NewManager(database.NewStore(db))
}

func TestCreateAndListBackups(t *testing.T) {
	mgr := newManager(t)
	db, err := mgr.Create("mydb", "default", string(database.EnginePostgres), "16", "", 5432)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	bk, err := mgr.CreateBackup(db.Name, "default", "full", func(d database.ManagedDB) (string, int64, error) {
		return "/backups/" + d.ID + ".dump", 1024, nil
	})
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if bk.Status != "complete" {
		t.Errorf("backup status: want complete got %q", bk.Status)
	}
	if bk.SizeBytes != 1024 {
		t.Errorf("backup size: want 1024 got %d", bk.SizeBytes)
	}

	backups, err := mgr.ListBackups(db.Name, "default")
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].DBID != db.ID {
		t.Errorf("backup DBID mismatch: want %q got %q", db.ID, backups[0].DBID)
	}
}

func TestBackupExecutorFailure(t *testing.T) {
	mgr := newManager(t)
	db, _ := mgr.Create("faildb", "default", string(database.EnginePostgres), "16", "", 5432)
	_, err := mgr.CreateBackup(db.Name, "default", "full", func(d database.ManagedDB) (string, int64, error) {
		return "", 0, fmt.Errorf("disk full")
	})
	if err == nil {
		t.Fatal("expected error from failing executor")
	}
	backups, _ := mgr.ListBackups(db.Name, "default")
	if len(backups) != 1 || backups[0].Status != "failed" {
		t.Errorf("expected 1 failed backup, got %v", backups)
	}
}

func TestDeleteBackup(t *testing.T) {
	mgr := newManager(t)
	db, _ := mgr.Create("deldb", "default", string(database.EnginePostgres), "16", "", 5432)
	bk, _ := mgr.CreateBackup(db.Name, "default", "full", func(d database.ManagedDB) (string, int64, error) {
		return "/backups/x.dump", 512, nil
	})
	if err := mgr.DeleteBackup(bk.ID); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	backups, _ := mgr.ListBackups(db.Name, "default")
	if len(backups) != 0 {
		t.Errorf("expected 0 backups after delete, got %d", len(backups))
	}
}

func TestDefaultPortsAndVersions(t *testing.T) {
	tests := []struct {
		engine  database.DBEngine
		port    int
		version string
	}{
		{database.EnginePostgres, 5432, "16"},
		{database.EngineRedis, 6379, "7"},
		{database.EngineMariaDB, 3306, "11"},
	}
	for _, tt := range tests {
		if got := database.DefaultPorts[tt.engine]; got != tt.port {
			t.Errorf("%s port: want %d got %d", tt.engine, tt.port, got)
		}
		if got := database.DefaultVersions[tt.engine]; got != tt.version {
			t.Errorf("%s version: want %q got %q", tt.engine, tt.version, got)
		}
	}
}

func TestRestoreIntoNewCreatesRunningTargetAfterRestore(t *testing.T) {
	mgr := newManager(t)
	var gotBackup, gotProject, gotConn string
	db, err := mgr.RestoreIntoNew(
		"bkp_1",
		"app-restored",
		"default",
		string(database.EnginePostgres),
		"16",
		"appnet",
		5432,
		"postgres://target/app-restored",
		func(backupID, project, targetConnectionString string) error {
			gotBackup = backupID
			gotProject = project
			gotConn = targetConnectionString
			return nil
		},
	)
	if err != nil {
		t.Fatalf("RestoreIntoNew: %v", err)
	}
	if gotBackup != "bkp_1" || gotProject != "default" || gotConn != "postgres://target/app-restored" {
		t.Fatalf("restore executor got backup=%q project=%q conn=%q", gotBackup, gotProject, gotConn)
	}
	if db.Name != "app-restored" || db.Status != database.DBStatusRunning {
		t.Fatalf("unexpected restored db: %+v", db)
	}
}
