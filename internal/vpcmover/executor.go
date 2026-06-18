package vpcmover

import (
	"context"
	"fmt"
	"time"
)

// EventRecorder records mobility audit events.
type EventRecorder interface {
	RecordMobilityEvent(ctx context.Context, action, jobID, details string)
}

type noopRecorder struct{}

func (n *noopRecorder) RecordMobilityEvent(_ context.Context, _, _, _ string) {}

// Executor runs a MobilityJob step by step.
type Executor struct {
	store    *Store
	invStore InventoryStore
	recorder EventRecorder
}

// NewExecutor creates an Executor backed by the given Store.
func NewExecutor(store *Store) *Executor {
	return &Executor{store: store, recorder: &noopRecorder{}}
}

// WithRecorder attaches an event recorder and returns the executor (fluent).
func (e *Executor) WithRecorder(rec EventRecorder) *Executor {
	e.recorder = rec
	return e
}

// WithInventoryStore attaches an InventoryStore used by validation steps.
func (e *Executor) WithInventoryStore(inv InventoryStore) *Executor {
	e.invStore = inv
	return e
}

// Execute runs all pending/retrying steps of the job in order.
// It is safe to call concurrently for different jobs.
func (e *Executor) Execute(ctx context.Context, job MobilityJob) error {
	steps, err := e.store.ListSteps(job.ID)
	if err != nil {
		return fmt.Errorf("list steps for job %s: %w", job.ID, err)
	}

	total := len(steps)
	for i, step := range steps {
		if step.Status != StepStatusPending && step.Status != StepStatusRetrying {
			continue
		}

		// Check for context cancellation before starting each step.
		select {
		case <-ctx.Done():
			// Mark as pending (paused) so we can resume later.
			_ = e.store.UpdateStepStatus(step.ID, StepStatusPending, "")
			_ = e.store.UpdateJobStatus(job.ID, JobStatusQueued, step.Name, progressPct(i, total))
			return ctx.Err()
		default:
		}

		// Mark step running.
		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = e.store.db.Exec(
			`UPDATE vpc_mobility_steps SET status='running', started_at=?, updated_at=? WHERE id=?`,
			now, now, step.ID,
		)
		_ = e.store.UpdateJobStatus(job.ID, JobStatusRunning, step.Name, progressPct(i, total))

		e.recorder.RecordMobilityEvent(ctx, "step-start", job.ID,
			fmt.Sprintf("step %s starting (order=%d)", step.Name, step.StepOrder))

		stepErr := e.dispatchStep(ctx, job, step)

		if stepErr == nil {
			if err := e.store.UpdateStepStatus(step.ID, StepStatusCompleted, ""); err != nil {
				return fmt.Errorf("mark step completed: %w", err)
			}
			e.recorder.RecordMobilityEvent(ctx, "step-done", job.ID,
				fmt.Sprintf("step %s completed", step.Name))
			continue
		}

		// Step failed.
		e.recorder.RecordMobilityEvent(ctx, "step-error", job.ID,
			fmt.Sprintf("step %s failed: %v", step.Name, stepErr))

		const maxRetries = 3
		if step.RetryCount < maxRetries {
			step.RetryCount++
			_, _ = e.store.db.Exec(
				`UPDATE vpc_mobility_steps SET status='retrying', retry_count=?, error_message=?, updated_at=? WHERE id=?`,
				step.RetryCount, stepErr.Error(), time.Now().UTC().Format(time.RFC3339), step.ID,
			)
			_ = e.store.UpdateJobStatus(job.ID, JobStatusRunning, step.Name, progressPct(i, total))
			// Re-run the step immediately (simple retry without backoff).
			stepErr2 := e.dispatchStep(ctx, job, step)
			if stepErr2 == nil {
				_ = e.store.UpdateStepStatus(step.ID, StepStatusCompleted, "")
				continue
			}
			stepErr = stepErr2
		}

		// Exhausted retries — fail the step and the job.
		_ = e.store.UpdateStepStatus(step.ID, StepStatusFailed, stepErr.Error())
		_ = e.store.FailJob(job.ID, fmt.Sprintf("step %q failed: %v", step.Name, stepErr))
		return fmt.Errorf("job %s failed at step %q: %w", job.ID, step.Name, stepErr)
	}

	// All steps done.
	if err := e.store.CompleteJob(job.ID); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	e.recorder.RecordMobilityEvent(ctx, "job-complete", job.ID, "all steps completed")
	return nil
}

func progressPct(current, total int) int {
	if total == 0 {
		return 100
	}
	return (current * 100) / total
}
