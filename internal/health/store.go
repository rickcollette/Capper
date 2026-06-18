package health

import (
	"database/sql"
	"fmt"
	"strings"
)

// Store persists health check results.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS instance_health_checks (
		instance_id TEXT PRIMARY KEY,
		status      TEXT NOT NULL DEFAULT 'unknown',
		message     TEXT NOT NULL DEFAULT '',
		checked_at  TEXT NOT NULL DEFAULT ''
	)`)
	return err
}

func (s *Store) Upsert(r Result) error {
	_, err := s.db.Exec(
		`INSERT INTO instance_health_checks(instance_id, status, message, checked_at)
		 VALUES(?,?,?,?)
		 ON CONFLICT(instance_id) DO UPDATE SET
		   status=excluded.status, message=excluded.message, checked_at=excluded.checked_at`,
		r.InstanceID, r.Status, r.Message, r.CheckedAt,
	)
	return err
}

func (s *Store) Get(instanceID string) (Result, error) {
	var r Result
	err := s.db.QueryRow(
		`SELECT instance_id, status, message, checked_at FROM instance_health_checks WHERE instance_id=?`,
		instanceID,
	).Scan(&r.InstanceID, &r.Status, &r.Message, &r.CheckedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Result{}, fmt.Errorf("health: no check for instance %q", instanceID)
		}
		return Result{}, err
	}
	return r, nil
}

func (s *Store) List() ([]Result, error) {
	rows, err := s.db.Query(
		`SELECT instance_id, status, message, checked_at FROM instance_health_checks ORDER BY checked_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.InstanceID, &r.Status, &r.Message, &r.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
