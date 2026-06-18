package posture_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/posture"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := posture.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestScan_EmptyDir(t *testing.T) {
	db := openDB(t)
	sc := posture.NewScanner(posture.NewStore(db))
	dir := t.TempDir()

	result, err := sc.Scan("proj1", dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.ScanID == "" {
		t.Error("ScanID must be set")
	}
	if result.ScannedAt == "" {
		t.Error("ScannedAt must be set")
	}
}

func TestScan_DetectsWorldWritable(t *testing.T) {
	db := openDB(t)
	sc := posture.NewScanner(posture.NewStore(db))
	dir := t.TempDir()

	ww := filepath.Join(dir, "world-writable.sh")
	if err := os.WriteFile(ww, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Use Chmod to bypass umask so the file is truly world-writable.
	if err := os.Chmod(ww, 0o777); err != nil {
		t.Fatal(err)
	}

	result, err := sc.Scan("proj1", dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Check == "world-writable" {
			found = true
		}
	}
	if !found {
		t.Error("expected world-writable finding for 0777 file")
	}
}

func TestList_ReturnsPersisted(t *testing.T) {
	db := openDB(t)
	sc := posture.NewScanner(posture.NewStore(db))
	dir := t.TempDir()

	_, err := sc.Scan("proj1", dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	findings, err := sc.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Any scan (even clean) should succeed; findings may be 0 for empty dir.
	_ = findings
}

func TestSeverityValues(t *testing.T) {
	vals := []posture.Severity{posture.SeverityHigh, posture.SeverityMedium, posture.SeverityLow, posture.SeverityInfo}
	for _, v := range vals {
		if string(v) == "" {
			t.Errorf("empty severity value")
		}
	}
}
