package stack

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// JobStatus tracks execution state of a job run.
type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// JobSpec is the parsed structure of a job JSON/YAML file.
type JobSpec struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Steps []JobStep `json:"steps"`
	} `json:"spec"`
}

// JobStep is one executable step within a job.
type JobStep struct {
	Run string `json:"run"`
}

// Job is a stored job record.
type Job struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Project   string    `json:"project"`
	SpecYAML  string    `json:"specYaml"`
	Status    JobStatus `json:"status"`
	Logs      string    `json:"logs"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
}

// InitJobSchema creates the jobs table alongside the stacks table.
func InitJobSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS jobs (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		project    TEXT NOT NULL DEFAULT '',
		spec_yaml  TEXT NOT NULL DEFAULT '',
		status     TEXT NOT NULL DEFAULT 'queued',
		logs       TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE(name, project)
	)`)
	return err
}

// JobStore persists jobs in SQLite.
type JobStore struct{ db *sql.DB }

func NewJobStore(db *sql.DB) *JobStore { return &JobStore{db: db} }

func (s *JobStore) Insert(j Job) error {
	_, err := s.db.Exec(
		`INSERT INTO jobs(id, name, project, spec_yaml, status, logs, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		j.ID, j.Name, j.Project, j.SpecYAML, string(j.Status), j.Logs, j.CreatedAt, j.UpdatedAt,
	)
	return err
}

func (s *JobStore) Get(nameOrID, project string) (Job, error) {
	var row *sql.Row
	if project != "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, spec_yaml, status, logs, created_at, updated_at
			 FROM jobs WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, spec_yaml, status, logs, created_at, updated_at
			 FROM jobs WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	}
	return scanJob(row)
}

func (s *JobStore) List(project string) ([]Job, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project != "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, spec_yaml, status, logs, created_at, updated_at
			 FROM jobs WHERE project=? ORDER BY created_at DESC`,
			project,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, spec_yaml, status, logs, created_at, updated_at
			 FROM jobs ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *JobStore) UpdateStatus(id string, status JobStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE jobs SET status=?, updated_at=? WHERE id=?`, string(status), now, id)
	return err
}

func (s *JobStore) AppendLog(id, line string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE jobs SET logs=logs||?, updated_at=? WHERE id=?`,
		line+"\n", now, id,
	)
	return err
}

func (s *JobStore) Delete(nameOrID, project string) error {
	if project != "" {
		_, err := s.db.Exec(`DELETE FROM jobs WHERE (id=? OR name=?) AND project=?`, nameOrID, nameOrID, project)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM jobs WHERE id=? OR name=?`, nameOrID, nameOrID)
	return err
}

// ParseJobSpec parses a job JSON document.
func ParseJobSpec(raw string) (JobSpec, error) {
	var spec JobSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return JobSpec{}, fmt.Errorf("job spec parse: %w", err)
	}
	if spec.Kind != "Job" {
		return JobSpec{}, fmt.Errorf("job spec: expected kind Job, got %q", spec.Kind)
	}
	if spec.Metadata.Name == "" {
		return JobSpec{}, fmt.Errorf("job spec: metadata.name is required")
	}
	if len(spec.Spec.Steps) == 0 {
		return JobSpec{}, fmt.Errorf("job spec: at least one step is required")
	}
	return spec, nil
}

// RunJob executes a job's steps in order, appending output to the job's log.
// It updates status to running/done/failed in the store.
func RunJob(store *JobStore, job Job) error {
	spec, err := ParseJobSpec(job.SpecYAML)
	if err != nil {
		_ = store.UpdateStatus(job.ID, JobFailed)
		_ = store.AppendLog(job.ID, "ERROR: "+err.Error())
		return err
	}
	_ = store.UpdateStatus(job.ID, JobRunning)

	for i, step := range spec.Spec.Steps {
		if step.Run == "" {
			continue
		}
		line := fmt.Sprintf("[step %d] $ %s", i+1, step.Run)
		_ = store.AppendLog(job.ID, line)

		out, runErr := runShellStep(step.Run)
		if out != "" {
			_ = store.AppendLog(job.ID, strings.TrimRight(out, "\n"))
		}
		if runErr != nil {
			_ = store.AppendLog(job.ID, "ERROR: "+runErr.Error())
			_ = store.UpdateStatus(job.ID, JobFailed)
			return fmt.Errorf("step %d failed: %w", i+1, runErr)
		}
	}

	_ = store.UpdateStatus(job.ID, JobDone)
	return nil
}

func runShellStep(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func scanJob(r rowScanner) (Job, error) {
	var j Job
	var status string
	err := r.Scan(&j.ID, &j.Name, &j.Project, &j.SpecYAML, &status, &j.Logs, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Job{}, fmt.Errorf("job not found")
		}
		return Job{}, err
	}
	j.Status = JobStatus(status)
	return j, nil
}
