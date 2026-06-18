package backup

import (
	"database/sql"
	"fmt"
	"time"
)

// Store persists backup records and policies in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the backup tables if they do not exist.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS backup_records (
			id         TEXT PRIMARY KEY,
			type       TEXT NOT NULL,
			project    TEXT NOT NULL DEFAULT 'default',
			path       TEXT NOT NULL,
			size_bytes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS backup_policies (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			project       TEXT NOT NULL DEFAULT 'default',
			type          TEXT NOT NULL,
			target_path   TEXT NOT NULL,
			source        TEXT NOT NULL DEFAULT '',
			interval_secs INTEGER NOT NULL DEFAULT 0,
			retention     INTEGER NOT NULL DEFAULT 0,
			last_run_at   TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	_, _ = db.Exec(`ALTER TABLE backup_policies ADD COLUMN source TEXT NOT NULL DEFAULT ''`)
	return nil
}

// --- records ---

func (s *Store) InsertRecord(r BackupRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO backup_records (id, type, project, path, size_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.Type, r.Project, r.Path, r.SizeBytes, r.CreatedAt,
	)
	return err
}

func (s *Store) ListRecords(project string) ([]BackupRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, type, project, path, size_bytes, created_at
			 FROM backup_records ORDER BY created_at DESC`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, type, project, path, size_bytes, created_at
			 FROM backup_records WHERE project=? ORDER BY created_at DESC`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupRecord
	for rows.Next() {
		var r BackupRecord
		if err := rows.Scan(&r.ID, &r.Type, &r.Project, &r.Path, &r.SizeBytes, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRecord(id string) error {
	_, err := s.db.Exec(`DELETE FROM backup_records WHERE id=?`, id)
	return err
}

// --- policies ---

func (s *Store) InsertPolicy(p Policy) error {
	_, err := s.db.Exec(
		`INSERT INTO backup_policies (id, name, project, type, target_path, source, interval_secs, retention, last_run_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Project, p.Type, p.TargetPath, p.Source, p.IntervalSecs, p.Retention, p.LastRunAt, p.CreatedAt,
	)
	return err
}

func (s *Store) GetPolicy(nameOrID, project string) (Policy, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, type, target_path, source, interval_secs, retention, last_run_at, created_at
			 FROM backup_policies WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, type, target_path, source, interval_secs, retention, last_run_at, created_at
			 FROM backup_policies WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanPolicy(row)
}

func (s *Store) ListPolicies(project string) ([]Policy, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, type, target_path, source, interval_secs, retention, last_run_at, created_at
			 FROM backup_policies ORDER BY name`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, type, target_path, source, interval_secs, retention, last_run_at, created_at
			 FROM backup_policies WHERE project=? ORDER BY name`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Policy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DeletePolicy(nameOrID, project string) error {
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(`DELETE FROM backup_policies WHERE id=? OR name=?`, nameOrID, nameOrID)
	} else {
		res, err = s.db.Exec(`DELETE FROM backup_policies WHERE (id=? OR name=?) AND project=?`,
			nameOrID, nameOrID, project)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("backup policy %q not found", nameOrID)
	}
	return nil
}

func (s *Store) UpdatePolicyLastRun(id, lastRunAt string) error {
	_, err := s.db.Exec(`UPDATE backup_policies SET last_run_at=? WHERE id=?`, lastRunAt, id)
	return err
}

// ListDuePolicies returns policies whose interval has elapsed since last run.
func (s *Store) ListDuePolicies() ([]Policy, error) {
	rows, err := s.db.Query(
		`SELECT id, name, project, type, target_path, source, interval_secs, retention, last_run_at, created_at
		 FROM backup_policies WHERE interval_secs > 0`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	var out []Policy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		if p.LastRunAt == "" {
			out = append(out, p)
			continue
		}
		last, perr := time.Parse(time.RFC3339, p.LastRunAt)
		if perr != nil {
			out = append(out, p)
			continue
		}
		if now.Sub(last) >= time.Duration(p.IntervalSecs)*time.Second {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func scanPolicy(s rowScanner) (Policy, error) {
	var p Policy
	if err := s.Scan(&p.ID, &p.Name, &p.Project, &p.Type, &p.TargetPath, &p.Source,
		&p.IntervalSecs, &p.Retention, &p.LastRunAt, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Policy{}, fmt.Errorf("backup policy not found")
		}
		return Policy{}, fmt.Errorf("backup: scan: %w", err)
	}
	return p, nil
}

func newID() string {
	return "bkp_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
