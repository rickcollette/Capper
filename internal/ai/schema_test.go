package ai_test

import (
	"testing"

	"capper/internal/ai"
)

func TestSecureAITablesExist(t *testing.T) {
	db := openDB(t)
	_ = db

	tables := []string{
		"ai_approval_gates",
		"ai_assumed_roles",
		"ai_ledger",
		"ai_policies",
	}
	for _, tbl := range tables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %s must exist after InitSchema: %v", tbl, err)
		}
	}
}

func TestInitSchemaIdempotent(t *testing.T) {
	db := openDB(t)
	// Second call must be safe (CREATE TABLE IF NOT EXISTS).
	if err := ai.InitSchema(db); err != nil {
		t.Fatalf("second InitSchema call failed: %v", err)
	}
}
