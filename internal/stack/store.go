package stack

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS stacks (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL UNIQUE,
		project       TEXT NOT NULL DEFAULT '',
		template_hash TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'active',
		resources     TEXT NOT NULL DEFAULT '[]',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	)`)
	return err
}

func (s *Store) Insert(st Stack) error {
	res, _ := json.Marshal(st.Resources)
	_, err := s.db.Exec(
		`INSERT INTO stacks(id, name, project, template_hash, status, resources, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		st.ID, st.Name, st.Project, st.TemplateHash, string(st.Status), string(res), st.CreatedAt, st.UpdatedAt,
	)
	return err
}

func (s *Store) Get(nameOrID, project string) (Stack, error) {
	var row *sql.Row
	if project != "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, template_hash, status, resources, created_at, updated_at
			 FROM stacks WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, template_hash, status, resources, created_at, updated_at
			 FROM stacks WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	}
	return scanStack(row)
}

func (s *Store) List(project string) ([]Stack, error) {
	var rows *sql.Rows
	var err error
	if project != "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, template_hash, status, resources, created_at, updated_at
			 FROM stacks WHERE project=? ORDER BY created_at`,
			project,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, template_hash, status, resources, created_at, updated_at
			 FROM stacks ORDER BY created_at`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Stack
	for rows.Next() {
		st, err := scanStack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateResources(id string, resources []StackResource) error {
	res, _ := json.Marshal(resources)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE stacks SET resources=?, updated_at=? WHERE id=?`,
		string(res), now, id,
	)
	return err
}

func (s *Store) UpdateStatus(id string, status StackStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE stacks SET status=?, updated_at=? WHERE id=?`,
		string(status), now, id,
	)
	return err
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM stacks WHERE id=?`, id)
	return err
}

type rowScanner interface{ Scan(dest ...any) error }

func scanStack(r rowScanner) (Stack, error) {
	var st Stack
	var status, resources string
	err := r.Scan(&st.ID, &st.Name, &st.Project, &st.TemplateHash, &status, &resources, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Stack{}, fmt.Errorf("stack not found")
		}
		return Stack{}, err
	}
	st.Status = StackStatus(status)
	_ = json.Unmarshal([]byte(resources), &st.Resources)
	return st, nil
}
