package store

import (
	"fmt"
	"sort"
	"time"
)

// knownMigrations is the canonical, ordered list of named schema migrations the
// current binary knows how to apply. It is the source of truth for reporting
// pending vs applied migrations (the additive column block in Open is folded in
// here as it is converted to numbered migrations). Keep this in lockstep with the
// applyMigration calls in Open.
var knownMigrations = []string{
	"0001_tenancy_columns",
}

// AppliedMigration is one row of the schema_migrations bookkeeping table.
type AppliedMigration struct {
	Version   string `json:"version"`
	AppliedAt string `json:"appliedAt"`
}

// SchemaVersion returns the latest applied migration version (lexically highest,
// which matches the zero-padded numbering), or "" when none have been applied.
func (s *Store) SchemaVersion() (string, error) {
	applied, err := s.AppliedMigrations()
	if err != nil {
		return "", err
	}
	if len(applied) == 0 {
		return "", nil
	}
	return applied[len(applied)-1].Version, nil
}

// AppliedMigrations returns every recorded migration, ordered by version.
func (s *Store) AppliedMigrations() ([]AppliedMigration, error) {
	rows, err := s.DB.Query(`SELECT version, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("store: list migrations: %w", err)
	}
	defer rows.Close()
	var out []AppliedMigration
	for rows.Next() {
		var m AppliedMigration
		if err := rows.Scan(&m.Version, &m.AppliedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PendingMigrations returns known migrations not yet recorded as applied.
func (s *Store) PendingMigrations() ([]string, error) {
	applied, err := s.AppliedMigrations()
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(applied))
	for _, m := range applied {
		have[m.Version] = true
	}
	var pending []string
	for _, v := range knownMigrations {
		if !have[v] {
			pending = append(pending, v)
		}
	}
	sort.Strings(pending)
	return pending, nil
}

// SnapshotDB writes a consistent, online snapshot of the control-plane database
// to destPath using SQLite's `VACUUM INTO`. This works for both the pure-Go
// SQLite backend (writes a local file) and the CapDB backend (the server writes
// the file relative to its --db-root). It is safe to run on a live database.
// Callers (upgrade/migrate) take a snapshot here before applying migrations so a
// failed upgrade can be rolled back.
func (s *Store) SnapshotDB(destPath string) error {
	if destPath == "" {
		return fmt.Errorf("store: snapshot destination path is empty")
	}
	// VACUUM INTO refuses to overwrite an existing file.
	if _, err := s.DB.Exec(`VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("store: snapshot to %q failed: %w", destPath, err)
	}
	return nil
}

// SnapshotPath returns a conventional timestamped snapshot path next to the DB
// file (pure-Go backend). For the CapDB backend the path is interpreted
// server-side.
func (s *Store) SnapshotPath() string {
	return fmt.Sprintf("%s.backup-%s", s.Paths.DB, time.Now().UTC().Format("20060102T150405Z"))
}
