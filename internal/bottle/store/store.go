package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"capper/internal/bottle"
)

// Store manages the bottle, deployment, and job tables.
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates all bottle-related tables.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bottles (
			id           TEXT PRIMARY KEY,
			project      TEXT NOT NULL DEFAULT '',
			name         TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			version      TEXT NOT NULL,
			description  TEXT NOT NULL DEFAULT '',
			author       TEXT NOT NULL DEFAULT '',
			license      TEXT NOT NULL DEFAULT '',
			source       TEXT NOT NULL DEFAULT '',
			digest       TEXT NOT NULL DEFAULT '',
			raw_json     TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'active',
			tags_json    TEXT NOT NULL DEFAULT '[]',
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL,
			UNIQUE(project, name, version)
		)`,
		`CREATE TABLE IF NOT EXISTS bottle_deployments (
			id              TEXT PRIMARY KEY,
			project         TEXT NOT NULL,
			bottle_id       TEXT NOT NULL,
			name            TEXT NOT NULL,
			version         TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'planning',
			parameters_json TEXT NOT NULL DEFAULT '{}',
			outputs_json    TEXT NOT NULL DEFAULT '{}',
			resources_json  TEXT NOT NULL DEFAULT '[]',
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL,
			UNIQUE(project, name),
			FOREIGN KEY(bottle_id) REFERENCES bottles(id)
		)`,
		`CREATE TABLE IF NOT EXISTS bottle_jobs (
			id            TEXT PRIMARY KEY,
			project       TEXT NOT NULL DEFAULT '',
			deployment_id TEXT NOT NULL DEFAULT '',
			bottle_id     TEXT NOT NULL DEFAULT '',
			job_type      TEXT NOT NULL,
			status        TEXT NOT NULL DEFAULT 'queued',
			logs          TEXT NOT NULL DEFAULT '',
			result_json   TEXT NOT NULL DEFAULT '{}',
			created_at    TEXT NOT NULL,
			updated_at    TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// ---- Bottle CRUD -----------------------------------------------------------

func (s *Store) InsertBottle(b bottle.Bottle) error {
	tags, _ := json.Marshal(b.Tags)
	_, err := s.db.Exec(
		`INSERT INTO bottles(id,project,name,display_name,version,description,author,license,source,digest,raw_json,status,tags_json,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.Project, b.Name, b.DisplayName, b.Version, b.Description,
		b.Author, b.License, b.Source, b.Digest, b.RawJSON,
		string(b.Status), string(tags), b.CreatedAt, b.UpdatedAt,
	)
	return err
}

func (s *Store) GetBottle(nameOrID, project string) (bottle.Bottle, error) {
	var row *sql.Row
	if project != "" {
		row = s.db.QueryRow(
			`SELECT id,project,name,display_name,version,description,author,license,source,digest,raw_json,status,tags_json,created_at,updated_at
			 FROM bottles WHERE (id=? OR name=?) AND project=? ORDER BY created_at DESC LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id,project,name,display_name,version,description,author,license,source,digest,raw_json,status,tags_json,created_at,updated_at
			 FROM bottles WHERE id=? OR name=? ORDER BY created_at DESC LIMIT 1`,
			nameOrID, nameOrID,
		)
	}
	return scanBottle(row)
}

func (s *Store) ListBottles(project string) ([]bottle.Bottle, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project != "" {
		rows, err = s.db.Query(
			`SELECT id,project,name,display_name,version,description,author,license,source,digest,raw_json,status,tags_json,created_at,updated_at
			 FROM bottles WHERE project=? ORDER BY name,version`,
			project,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id,project,name,display_name,version,description,author,license,source,digest,raw_json,status,tags_json,created_at,updated_at
			 FROM bottles ORDER BY name,version`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bottle.Bottle
	for rows.Next() {
		b, err := scanBottle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBottle(id string) error {
	_, err := s.db.Exec(`DELETE FROM bottles WHERE id=?`, id)
	return err
}

// ---- Deployment CRUD -------------------------------------------------------

func (s *Store) InsertDeployment(d bottle.BottleDeployment) error {
	params, _ := json.Marshal(d.Parameters)
	outputs, _ := json.Marshal(d.Outputs)
	resources, _ := json.Marshal(d.Resources)
	_, err := s.db.Exec(
		`INSERT INTO bottle_deployments(id,project,bottle_id,name,version,status,parameters_json,outputs_json,resources_json,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.Project, d.BottleID, d.Name, d.Version,
		string(d.Status), string(params), string(outputs), string(resources),
		d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (s *Store) GetDeployment(nameOrID, project string) (bottle.BottleDeployment, error) {
	var row *sql.Row
	if project != "" {
		row = s.db.QueryRow(
			`SELECT id,project,bottle_id,name,version,status,parameters_json,outputs_json,resources_json,created_at,updated_at
			 FROM bottle_deployments WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id,project,bottle_id,name,version,status,parameters_json,outputs_json,resources_json,created_at,updated_at
			 FROM bottle_deployments WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	}
	return scanDeployment(row)
}

func (s *Store) ListDeployments(project string) ([]bottle.BottleDeployment, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project != "" {
		rows, err = s.db.Query(
			`SELECT id,project,bottle_id,name,version,status,parameters_json,outputs_json,resources_json,created_at,updated_at
			 FROM bottle_deployments WHERE project=? ORDER BY name`,
			project,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id,project,bottle_id,name,version,status,parameters_json,outputs_json,resources_json,created_at,updated_at
			 FROM bottle_deployments ORDER BY name`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bottle.BottleDeployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDeploymentStatus(id string, status bottle.DeploymentStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE bottle_deployments SET status=?,updated_at=? WHERE id=?`, string(status), now, id)
	return err
}

func (s *Store) UpdateDeploymentOutputs(id string, outputs map[string]string, resources []bottle.DeployedResource) error {
	out, _ := json.Marshal(outputs)
	res, _ := json.Marshal(resources)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE bottle_deployments SET outputs_json=?,resources_json=?,updated_at=? WHERE id=?`,
		string(out), string(res), now, id,
	)
	return err
}

func (s *Store) DeleteDeployment(id string) error {
	_, err := s.db.Exec(`DELETE FROM bottle_deployments WHERE id=?`, id)
	return err
}

// ---- Job CRUD --------------------------------------------------------------

func (s *Store) InsertJob(j bottle.BottleJob) error {
	_, err := s.db.Exec(
		`INSERT INTO bottle_jobs(id,project,deployment_id,bottle_id,job_type,status,logs,result_json,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.Project, j.DeploymentID, j.BottleID,
		string(j.JobType), string(j.Status), j.Logs, j.ResultJSON,
		j.CreatedAt, j.UpdatedAt,
	)
	return err
}

func (s *Store) GetJob(id string) (bottle.BottleJob, error) {
	row := s.db.QueryRow(
		`SELECT id,project,deployment_id,bottle_id,job_type,status,logs,result_json,created_at,updated_at
		 FROM bottle_jobs WHERE id=? LIMIT 1`, id,
	)
	return scanJob(row)
}

func (s *Store) ListJobs(project string) ([]bottle.BottleJob, error) {
	rows, err := s.db.Query(
		`SELECT id,project,deployment_id,bottle_id,job_type,status,logs,result_json,created_at,updated_at
		 FROM bottle_jobs WHERE project=? ORDER BY created_at DESC`,
		project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bottle.BottleJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) UpdateJobStatus(id string, status bottle.JobStatus, logs string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE bottle_jobs SET status=?,logs=?,updated_at=? WHERE id=?`,
		string(status), logs, now, id,
	)
	return err
}

// ---- scanners --------------------------------------------------------------

type rowScanner interface{ Scan(dest ...any) error }

func scanBottle(r rowScanner) (bottle.Bottle, error) {
	var b bottle.Bottle
	var status, tagsJSON string
	err := r.Scan(
		&b.ID, &b.Project, &b.Name, &b.DisplayName, &b.Version,
		&b.Description, &b.Author, &b.License, &b.Source, &b.Digest,
		&b.RawJSON, &status, &tagsJSON, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return bottle.Bottle{}, fmt.Errorf("bottle not found")
		}
		return bottle.Bottle{}, err
	}
	b.Status = bottle.BottleStatus(status)
	_ = json.Unmarshal([]byte(tagsJSON), &b.Tags)
	return b, nil
}

func scanDeployment(r rowScanner) (bottle.BottleDeployment, error) {
	var d bottle.BottleDeployment
	var status, paramsJSON, outputsJSON, resourcesJSON string
	err := r.Scan(
		&d.ID, &d.Project, &d.BottleID, &d.Name, &d.Version,
		&status, &paramsJSON, &outputsJSON, &resourcesJSON,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return bottle.BottleDeployment{}, fmt.Errorf("deployment not found")
		}
		return bottle.BottleDeployment{}, err
	}
	d.Status = bottle.DeploymentStatus(status)
	_ = json.Unmarshal([]byte(paramsJSON), &d.Parameters)
	_ = json.Unmarshal([]byte(outputsJSON), &d.Outputs)
	_ = json.Unmarshal([]byte(resourcesJSON), &d.Resources)
	return d, nil
}

func scanJob(r rowScanner) (bottle.BottleJob, error) {
	var j bottle.BottleJob
	var jobType, status string
	err := r.Scan(
		&j.ID, &j.Project, &j.DeploymentID, &j.BottleID,
		&jobType, &status, &j.Logs, &j.ResultJSON,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return bottle.BottleJob{}, fmt.Errorf("bottle job not found")
		}
		return bottle.BottleJob{}, err
	}
	j.JobType = bottle.JobType(jobType)
	j.Status = bottle.JobStatus(status)
	return j, nil
}
