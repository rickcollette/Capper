package types

import "time"

// DeletionJob tracks the state of an async resource deletion operation.
// Clients can poll the job status to monitor progress and errors.
type DeletionJob struct {
	ID                 string            `json:"id"`
	Status             string            `json:"status"` // queued, pre_flight, running, completed, failed, cancelled
	ResourceType       string            `json:"resourceType"`
	ResourceID         string            `json:"resourceId"`
	ConfirmationToken  string            `json:"confirmationToken,omitempty"`
	Progress           int               `json:"progress"` // 0-100
	CurrentStep        string            `json:"currentStep,omitempty"`
	Steps              []string          `json:"steps"`              // all steps in order
	CompletedSteps     []string          `json:"completedSteps"`    // steps that finished successfully
	RemainingSteps     []string          `json:"remainingSteps"`    // steps still to execute
	Errors             []DeletionJobError `json:"errors,omitempty"` // errors encountered
	CreatedAt          time.Time         `json:"createdAt"`
	StartedAt          *time.Time        `json:"startedAt,omitempty"`
	CompletedAt        *time.Time        `json:"completedAt,omitempty"`
	ExpiresAt          time.Time         `json:"expiresAt"` // auto-cleanup after this time
}

// DeletionJobError describes an error that occurred during a deletion step.
type DeletionJobError struct {
	Step        string `json:"step"`       // step name where error occurred
	Resource    string `json:"resource"`   // resource type ("instance", "subnet", etc.)
	ResourceID  string `json:"resourceId"` // resource ID
	Reason      string `json:"reason"`     // error message
	Recoverable bool   `json:"recoverable"` // whether deletion can retry or must be skipped
	Recovery    string `json:"recovery"`   // suggested recovery action
}

// DeletionJobStore defines storage operations for deletion jobs.
type DeletionJobStore interface {
	Create(job *DeletionJob) error
	Get(jobID string) (*DeletionJob, error)
	UpdateProgress(jobID, currentStep string, completed, remaining []string, percent int) error
	AddError(jobID string, err DeletionJobError) error
	Complete(jobID string, success bool) error
	Cancel(jobID string) error
	PruneExpired() error
}
