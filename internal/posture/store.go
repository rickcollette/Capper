package posture

import (
	"database/sql"
	"fmt"
	"time"
)

// Store persists posture findings in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the posture_findings table if it does not exist.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS posture_findings (
		id         TEXT PRIMARY KEY,
		scan_id    TEXT NOT NULL,
		project    TEXT NOT NULL DEFAULT 'default',
		check_name TEXT NOT NULL,
		severity   TEXT NOT NULL,
		target     TEXT NOT NULL,
		detail     TEXT NOT NULL,
		scanned_at TEXT NOT NULL
	)`)
	return err
}

func (s *Store) InsertAll(findings []Finding) error {
	for _, f := range findings {
		_, err := s.db.Exec(
			`INSERT INTO posture_findings (id, scan_id, project, check_name, severity, target, detail, scanned_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			f.ID, f.ScanID, f.Project, f.Check, f.Severity, f.Target, f.Detail, f.ScannedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListByProject returns findings ordered newest-first.
func (s *Store) ListByProject(project string) ([]Finding, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, scan_id, project, check_name, severity, target, detail, scanned_at
			 FROM posture_findings ORDER BY scanned_at DESC`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, scan_id, project, check_name, severity, target, detail, scanned_at
			 FROM posture_findings WHERE project=? ORDER BY scanned_at DESC`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.ScanID, &f.Project, &f.Check, &f.Severity, &f.Target, &f.Detail, &f.ScannedAt); err != nil {
			return nil, fmt.Errorf("posture: scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func newID() string {
	return "pos_" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func newScanID() string {
	return "scan_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
