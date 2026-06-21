package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"capper/internal/types"
)

// DeletionJobStore implements types.DeletionJobStore using SQLite.
type DeletionJobStore struct {
	db *sql.DB
}

// NewDeletionJobStore creates a new deletion job store.
func NewDeletionJobStore(db *sql.DB) *DeletionJobStore {
	return &DeletionJobStore{db: db}
}

// Create inserts a new deletion job into the store.
func (s *DeletionJobStore) Create(job *types.DeletionJob) error {
	stepsJSON, err := json.Marshal(job.Steps)
	if err != nil {
		return fmt.Errorf("failed to marshal steps: %w", err)
	}
	completedJSON, _ := json.Marshal([]string{})
	errorsJSON, _ := json.Marshal([]types.DeletionJobError{})

	result, err := s.db.Exec(`
		INSERT INTO deletion_jobs(
			id, status, resource_type, resource_id, confirmation_token,
			progress, current_step, steps, completed_steps, errors,
			created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.ID, job.Status, job.ResourceType, job.ResourceID, job.ConfirmationToken,
		job.Progress, job.CurrentStep, string(stepsJSON), string(completedJSON), string(errorsJSON),
		job.CreatedAt.Format(time.RFC3339), job.ExpiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows inserted")
	}

	return nil
}

// Get retrieves a deletion job by ID.
func (s *DeletionJobStore) Get(jobID string) (*types.DeletionJob, error) {
	row := s.db.QueryRow(`
		SELECT id, status, resource_type, resource_id, confirmation_token,
		       progress, current_step, steps, completed_steps, errors,
		       created_at, started_at, completed_at, expires_at
		FROM deletion_jobs WHERE id = ?
	`, jobID)

	var job types.DeletionJob
	var stepsJSON, completedJSON, errorsJSON sql.NullString
	var createdAtStr, startedAtStr, completedAtStr, expiresAtStr sql.NullString
	var statusStr sql.NullString

	err := row.Scan(
		&job.ID, &statusStr, &job.ResourceType, &job.ResourceID, &job.ConfirmationToken,
		&job.Progress, &job.CurrentStep, &stepsJSON, &completedJSON, &errorsJSON,
		&createdAtStr, &startedAtStr, &completedAtStr, &expiresAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if statusStr.Valid {
		job.Status = statusStr.String
	}

	// Parse timestamps
	if createdAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, createdAtStr.String)
		job.CreatedAt = t
	}
	if startedAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, startedAtStr.String)
		job.StartedAt = &t
	}
	if completedAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedAtStr.String)
		job.CompletedAt = &t
	}
	if expiresAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAtStr.String)
		job.ExpiresAt = t
	}

	// Unmarshal JSON arrays (with defaults if NULL)
	if stepsJSON.Valid {
		_ = json.Unmarshal([]byte(stepsJSON.String), &job.Steps)
	} else {
		job.Steps = []string{}
	}

	if completedJSON.Valid {
		_ = json.Unmarshal([]byte(completedJSON.String), &job.CompletedSteps)
	} else {
		job.CompletedSteps = []string{}
	}

	if errorsJSON.Valid {
		_ = json.Unmarshal([]byte(errorsJSON.String), &job.Errors)
	} else {
		job.Errors = []types.DeletionJobError{}
	}

	// Compute remaining steps
	job.RemainingSteps = make([]string, 0, len(job.Steps))
	completedMap := make(map[string]bool)
	for _, completed := range job.CompletedSteps {
		completedMap[completed] = true
	}
	for _, step := range job.Steps {
		if !completedMap[step] && step != job.CurrentStep {
			job.RemainingSteps = append(job.RemainingSteps, step)
		}
	}

	return &job, nil
}

// UpdateProgress updates the job's progress and current step.
func (s *DeletionJobStore) UpdateProgress(jobID, currentStep string, completed, remaining []string, percent int) error {
	completedJSON, _ := json.Marshal(completed)

	_, err := s.db.Exec(`
		UPDATE deletion_jobs
		SET progress = ?, current_step = ?, completed_steps = ?
		WHERE id = ?
	`, percent, currentStep, completedJSON, jobID)
	return err
}

// AddError adds an error to the job's error list.
func (s *DeletionJobStore) AddError(jobID string, err types.DeletionJobError) error {
	job, gerr := s.Get(jobID)
	if gerr != nil {
		return gerr
	}

	job.Errors = append(job.Errors, err)
	errorsJSON, _ := json.Marshal(job.Errors)

	_, dberr := s.db.Exec(`
		UPDATE deletion_jobs
		SET errors = ?
		WHERE id = ?
	`, errorsJSON, jobID)
	return dberr
}

// Complete marks the job as completed or failed.
func (s *DeletionJobStore) Complete(jobID string, success bool) error {
	status := "failed"
	if success {
		status = "completed"
	}

	_, err := s.db.Exec(`
		UPDATE deletion_jobs
		SET status = ?, progress = 100, completed_at = ?
		WHERE id = ?
	`, status, time.Now(), jobID)
	return err
}

// Cancel cancels a running deletion job.
func (s *DeletionJobStore) Cancel(jobID string) error {
	_, err := s.db.Exec(`
		UPDATE deletion_jobs
		SET status = 'cancelled'
		WHERE id = ? AND status IN ('queued', 'running')
	`, jobID)
	return err
}

// PruneExpired removes deletion jobs that have expired.
func (s *DeletionJobStore) PruneExpired() error {
	_, err := s.db.Exec(`
		DELETE FROM deletion_jobs WHERE expires_at < ?
	`, time.Now())
	return err
}
