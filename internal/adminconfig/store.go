// Package adminconfig persists admin-only platform settings as a small
// key/value store. It backs host-wide limits and other Admin-section config
// that does not belong to any single resource subsystem.
package adminconfig

import (
	"database/sql"
	"strconv"
	"time"
)

// Store persists admin configuration key/value pairs.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over an open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the admin_config table. Safe to call repeatedly.
func (s *Store) InitSchema() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS admin_config (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	return err
}

// Get returns the value for key and whether it was set.
func (s *Store) Get(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM admin_config WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// GetInt returns the value for key parsed as an int64 and whether it was set.
// A set-but-unparseable value is treated as unset.
func (s *Store) GetInt(key string) (int64, bool, error) {
	v, ok, err := s.Get(key)
	if err != nil || !ok {
		return 0, false, err
	}
	n, perr := strconv.ParseInt(v, 10, 64)
	if perr != nil {
		return 0, false, nil
	}
	return n, true, nil
}

// Set upserts a value for key.
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO admin_config (key, value, updated_at) VALUES (?,?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, time.Now().UTC().Format(time.RFC3339))
	return err
}

// SetInt upserts an int64 value for key.
func (s *Store) SetInt(key string, value int64) error {
	return s.Set(key, strconv.FormatInt(value, 10))
}

// Delete removes key (no error if absent).
func (s *Store) Delete(key string) error {
	_, err := s.db.Exec(`DELETE FROM admin_config WHERE key=?`, key)
	return err
}

// All returns every configured key/value pair.
func (s *Store) All() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM admin_config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
