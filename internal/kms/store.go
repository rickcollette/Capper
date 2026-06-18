package kms

import (
	"database/sql"
	"fmt"
	"time"
)

// Store persists KMS keys in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the kms_keys table if it does not exist.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS kms_keys (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		project       TEXT NOT NULL DEFAULT 'default',
		status        TEXT NOT NULL DEFAULT 'active',
		encrypted_key BLOB NOT NULL,
		created_at    TEXT NOT NULL,
		rotated_at    TEXT NOT NULL DEFAULT '',
		rotated_from  TEXT NOT NULL DEFAULT '',
		UNIQUE(name, project)
	)`)
	return err
}

func (s *Store) Insert(k Key) error {
	_, err := s.db.Exec(
		`INSERT INTO kms_keys (id, name, project, status, encrypted_key, created_at, rotated_at, rotated_from)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ID, k.Name, k.Project, k.Status, k.EncryptedKey, k.CreatedAt, k.RotatedAt, k.RotatedFrom,
	)
	return err
}

func (s *Store) Get(nameOrID, project string) (Key, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, status, encrypted_key, created_at, rotated_at, rotated_from
			 FROM kms_keys WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, status, encrypted_key, created_at, rotated_at, rotated_from
			 FROM kms_keys WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanKey(row)
}

func (s *Store) List(project string) ([]Key, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, status, encrypted_key, created_at, rotated_at, rotated_from
			 FROM kms_keys ORDER BY name, created_at`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, status, encrypted_key, created_at, rotated_at, rotated_from
			 FROM kms_keys WHERE project=? ORDER BY name, created_at`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Key
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// MarkRotated updates a key's status to "rotated" and sets rotated_at.
func (s *Store) MarkRotated(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE kms_keys SET status='rotated', rotated_at=? WHERE id=?`, now, id,
	)
	return err
}

// MarkRotatedByName works like MarkRotated but uses the (name, project) pair.
func (s *Store) MarkRotatedByName(name, project string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE kms_keys SET status='rotated', rotated_at=? WHERE name=? AND project=? AND status='active'`,
		now, name, project,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("kms: no active key named %q in project %q", name, project)
	}
	return nil
}

// Delete removes a key by name or ID from the given project.
func (s *Store) Delete(nameOrID, project string) error {
	res, err := s.db.Exec(
		`DELETE FROM kms_keys WHERE (id=? OR name=?) AND project=?`, nameOrID, nameOrID, project,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("kms key not found")
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanKey(s rowScanner) (Key, error) {
	var k Key
	if err := s.Scan(&k.ID, &k.Name, &k.Project, &k.Status, &k.EncryptedKey,
		&k.CreatedAt, &k.RotatedAt, &k.RotatedFrom); err != nil {
		if err == sql.ErrNoRows {
			return Key{}, fmt.Errorf("kms key not found")
		}
		return Key{}, fmt.Errorf("kms: scan: %w", err)
	}
	return k, nil
}

func newID() string {
	return "kms_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
