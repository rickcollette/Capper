package alert

import (
	"database/sql"
	"fmt"
	"time"
)

// Store persists alert rules in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the alert_rules table if it does not exist.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS alert_rules (
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		project      TEXT NOT NULL DEFAULT 'default',
		type         TEXT NOT NULL,
		event_action TEXT NOT NULL DEFAULT '',
		window_secs  INTEGER NOT NULL DEFAULT 60,
		threshold    INTEGER NOT NULL DEFAULT 1,
		metric_name  TEXT NOT NULL DEFAULT '',
		enabled      INTEGER NOT NULL DEFAULT 1,
		created_at   TEXT NOT NULL,
		UNIQUE(name, project)
	)`)
	return err
}

func (s *Store) Insert(r Rule) error {
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO alert_rules (id, name, project, type, event_action, window_secs, threshold, metric_name, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Project, r.Type, r.EventAction, r.WindowSecs, r.Threshold,
		r.MetricName, enabled, r.CreatedAt,
	)
	return err
}

func (s *Store) Get(nameOrID, project string) (Rule, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, type, event_action, window_secs, threshold, metric_name, enabled, created_at
			 FROM alert_rules WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, type, event_action, window_secs, threshold, metric_name, enabled, created_at
			 FROM alert_rules WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanRule(row)
}

func (s *Store) List(project string) ([]Rule, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, type, event_action, window_secs, threshold, metric_name, enabled, created_at
			 FROM alert_rules ORDER BY name`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, type, event_action, window_secs, threshold, metric_name, enabled, created_at
			 FROM alert_rules WHERE project=? ORDER BY name`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Delete(nameOrID, project string) error {
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(`DELETE FROM alert_rules WHERE id=? OR name=?`, nameOrID, nameOrID)
	} else {
		res, err = s.db.Exec(`DELETE FROM alert_rules WHERE (id=? OR name=?) AND project=?`, nameOrID, nameOrID, project)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alert rule %q not found", nameOrID)
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanRule(s rowScanner) (Rule, error) {
	var r Rule
	var enabled int
	if err := s.Scan(&r.ID, &r.Name, &r.Project, &r.Type, &r.EventAction, &r.WindowSecs,
		&r.Threshold, &r.MetricName, &enabled, &r.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Rule{}, fmt.Errorf("alert rule not found")
		}
		return Rule{}, fmt.Errorf("alert: scan: %w", err)
	}
	r.Enabled = enabled != 0
	return r, nil
}

func newID() string {
	return "alrt_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
