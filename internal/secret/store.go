package secret

import (
	"database/sql"
	"fmt"
	"time"
)

// Store persists encrypted secrets in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the secrets table if it does not exist.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS secrets (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		project     TEXT NOT NULL DEFAULT 'default',
		description TEXT NOT NULL DEFAULT '',
		ciphertext  BLOB NOT NULL,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		UNIQUE(name, project)
	)`)
	return err
}

func (s *Store) Insert(sec Secret) error {
	_, err := s.db.Exec(
		`INSERT INTO secrets (id, name, project, description, ciphertext, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sec.ID, sec.Name, sec.Project, sec.Description, sec.Ciphertext, sec.CreatedAt, sec.UpdatedAt,
	)
	return err
}

func (s *Store) Upsert(sec Secret) error {
	_, err := s.db.Exec(
		`INSERT INTO secrets (id, name, project, description, ciphertext, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name, project) DO UPDATE SET
		   description = excluded.description,
		   ciphertext  = excluded.ciphertext,
		   updated_at  = excluded.updated_at`,
		sec.ID, sec.Name, sec.Project, sec.Description, sec.Ciphertext, sec.CreatedAt, sec.UpdatedAt,
	)
	return err
}

func (s *Store) Get(nameOrID, project string) (Secret, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, description, ciphertext, created_at, updated_at
			 FROM secrets WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, description, ciphertext, created_at, updated_at
			 FROM secrets WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanSecret(row)
}

func (s *Store) List(project string) ([]Secret, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, description, ciphertext, created_at, updated_at
			 FROM secrets ORDER BY name`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, description, ciphertext, created_at, updated_at
			 FROM secrets WHERE project=? ORDER BY name`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Secret
	for rows.Next() {
		s, err := scanSecret(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *Store) Delete(nameOrID, project string) error {
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(`DELETE FROM secrets WHERE id=? OR name=?`, nameOrID, nameOrID)
	} else {
		res, err = s.db.Exec(`DELETE FROM secrets WHERE (id=? OR name=?) AND project=?`, nameOrID, nameOrID, project)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("secret %q not found", nameOrID)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSecret(s rowScanner) (Secret, error) {
	var sec Secret
	if err := s.Scan(&sec.ID, &sec.Name, &sec.Project, &sec.Description, &sec.Ciphertext, &sec.CreatedAt, &sec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Secret{}, fmt.Errorf("secret not found")
		}
		return Secret{}, fmt.Errorf("secret: scan: %w", err)
	}
	return sec, nil
}

func newID() string {
	return "sec_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
