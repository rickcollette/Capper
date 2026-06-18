package resource

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Store persists Resource rows into the shared SQLite database.
// It is initialized via InitSchema and then passed to managers that need it.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by the provided database connection.
// Callers must have already run InitSchema on the same db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema creates the resources table and applies any additive migrations.
// Safe to call multiple times; existing tables and columns are left intact.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS resources (
		id          TEXT PRIMARY KEY,
		type        TEXT NOT NULL,
		name        TEXT NOT NULL,
		project     TEXT NOT NULL DEFAULT '',
		owner       TEXT NOT NULL DEFAULT '',
		labels      TEXT NOT NULL DEFAULT '{}',
		annotations TEXT NOT NULL DEFAULT '{}',
		status      TEXT NOT NULL DEFAULT 'creating',
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("resource: create table: %w", err)
	}

	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS resources_type_project ON resources(type, project)`,
		`CREATE INDEX IF NOT EXISTS resources_name        ON resources(name)`,
	} {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("resource: create index: %w", err)
		}
	}
	return nil
}

// Register inserts a new resource row. CreatedAt and UpdatedAt are set to now
// if they are zero-valued.
func (s *Store) Register(r Resource) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	labels, err := marshalMap(r.Labels)
	if err != nil {
		return err
	}
	annotations, err := marshalMap(r.Annotations)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO resources(id,type,name,project,owner,labels,annotations,status,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Type, r.Name, r.Project, r.Owner,
		labels, annotations, r.Status, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

// UpdateStatus sets the status and bumps updated_at for the given resource ID.
func (s *Store) UpdateStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE resources SET status=?, updated_at=? WHERE id=?`,
		status, now, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("resource %q not found", id)
	}
	return nil
}

// UpdateLabels replaces all labels on the given resource.
func (s *Store) UpdateLabels(id string, labels map[string]string) error {
	encoded, err := marshalMap(labels)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE resources SET labels=?, updated_at=? WHERE id=?`,
		encoded, now, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("resource %q not found", id)
	}
	return nil
}

// Get returns the resource with the given ID, or an error if not found.
func (s *Store) Get(id string) (Resource, error) {
	row := s.db.QueryRow(
		`SELECT id,type,name,project,owner,labels,annotations,status,created_at,updated_at
		 FROM resources WHERE id=?`, id,
	)
	return scanRow(row)
}

// GetByName returns the first resource matching type+name+project.
func (s *Store) GetByName(resourceType, name, project string) (Resource, error) {
	row := s.db.QueryRow(
		`SELECT id,type,name,project,owner,labels,annotations,status,created_at,updated_at
		 FROM resources WHERE type=? AND name=? AND project=?`,
		resourceType, name, project,
	)
	return scanRow(row)
}

// List returns all resources of the given type in the given project.
// Pass "" for project to list across all projects.
func (s *Store) List(resourceType, project string) ([]Resource, error) {
	var rows *sql.Rows
	var err error
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id,type,name,project,owner,labels,annotations,status,created_at,updated_at
			 FROM resources WHERE type=? ORDER BY created_at`,
			resourceType,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id,type,name,project,owner,labels,annotations,status,created_at,updated_at
			 FROM resources WHERE type=? AND project=? ORDER BY created_at`,
			resourceType, project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// ListBySelector returns all resources of the given type in the given project
// whose labels match selector. An empty selector matches all.
func (s *Store) ListBySelector(resourceType, project string, selector map[string]string) ([]Resource, error) {
	all, err := s.List(resourceType, project)
	if err != nil {
		return nil, err
	}
	if len(selector) == 0 {
		return all, nil
	}
	var out []Resource
	for _, r := range all {
		if MatchLabels(selector, r.Labels) {
			out = append(out, r)
		}
	}
	return out, nil
}

// Delete removes the resource row with the given ID.
func (s *Store) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM resources WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("resource %q not found", id)
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

func marshalMap(m map[string]string) (string, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalMap(s string) (map[string]string, error) {
	if s == "" || s == "{}" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRow(s scanner) (Resource, error) {
	var r Resource
	var labels, annotations string
	err := s.Scan(&r.ID, &r.Type, &r.Name, &r.Project, &r.Owner,
		&labels, &annotations, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Resource{}, fmt.Errorf("resource not found: %w", err)
		}
		return Resource{}, err
	}
	r.Labels, err = unmarshalMap(labels)
	if err != nil {
		return Resource{}, err
	}
	r.Annotations, err = unmarshalMap(annotations)
	return r, err
}

func scanRows(rows *sql.Rows) ([]Resource, error) {
	var out []Resource
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
