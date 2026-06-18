package vpcmover

import (
	"context"
	"fmt"
)

// Runner orchestrates planning and execution of VPC mobility operations.
type Runner struct {
	planner  *Planner
	executor *Executor
	registry *JobRegistry
	store    *Store
}

// NewRunner creates a Runner backed by the given Store.
func NewRunner(store *Store) *Runner {
	return &Runner{
		planner:  NewPlanner(store),
		executor: NewExecutor(store),
		registry: NewJobRegistry(),
		store:    store,
	}
}

// Plan generates a MobilityPlan from the given request.
func (r *Runner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	return r.planner.Plan(req)
}

// ExecutePlan creates a job from an approved plan and executes it asynchronously.
// The caller receives the created job; execution continues in the background.
func (r *Runner) ExecutePlan(ctx context.Context, planID, createdBy string) (MobilityJob, error) {
	job, err := r.planner.StartJob(planID, createdBy)
	if err != nil {
		return MobilityJob{}, fmt.Errorf("start job: %w", err)
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	r.registry.Register(job.ID, cancel)

	go func() {
		defer r.registry.Remove(job.ID)
		_ = r.executor.Execute(jobCtx, job)
	}()

	return job, nil
}

// Cancel cancels a running job by jobID.
func (r *Runner) Cancel(jobID string) {
	r.registry.Cancel(jobID)
	_ = r.store.FailJob(jobID, "cancelled by user")
}

// Rollback generates rollback steps for a completed/failed job and runs them.
// Returns a new rollback job.
func (r *Runner) Rollback(ctx context.Context, jobID string) (MobilityJob, error) {
	orig, err := r.store.GetJob(jobID)
	if err != nil {
		return MobilityJob{}, fmt.Errorf("get job: %w", err)
	}

	rollbackJob := MobilityJob{
		PlanID:      orig.PlanID,
		OrgID:       orig.OrgID,
		AccountID:   orig.AccountID,
		SourceVPCID: orig.SourceVPCID,
		Operation:   orig.Operation,
		Status:      JobStatusQueued,
		CreatedBy:   orig.CreatedBy,
	}

	saved, err := r.store.CreateJob(rollbackJob)
	if err != nil {
		return MobilityJob{}, fmt.Errorf("create rollback job: %w", err)
	}

	// Create rollback steps: release any locks and clean up destination.
	rollbackSteps := []string{"cleanup-destination", "release-locks"}
	for i, name := range rollbackSteps {
		if _, err := r.store.CreateStep(MobilityStep{
			JobID:     saved.ID,
			StepOrder: i + 1,
			Name:      name,
		}); err != nil {
			return MobilityJob{}, fmt.Errorf("create rollback step %s: %w", name, err)
		}
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	r.registry.Register(saved.ID, cancel)

	go func() {
		defer r.registry.Remove(saved.ID)
		_ = r.executor.Execute(jobCtx, saved)
	}()

	return saved, nil
}
