package vpcmover

import (
	"encoding/json"
	"fmt"
	"time"
)

// Planner generates MobilityPlans without touching any infrastructure.
// It validates, builds an inventory, checks compatibility, and returns an
// ordered step list along with any warnings or blocking errors.
type Planner struct {
	store *Store
}

// NewPlanner creates a Planner backed by the given Store.
func NewPlanner(store *Store) *Planner {
	return &Planner{store: store}
}

// PlanRequest is the input to Plan().
type PlanRequest struct {
	OrgID          string
	AccountID      string
	ProjectID      string
	SourceVPCID    string
	TargetRealmID  string
	TargetRegionID string
	TargetZoneID   string
	Operation      Operation
	Options        PlanOptions
	CreatedBy      string
	// Inventory can be injected for testing (nil = planner builds a stub).
	Inventory *VPCInventory
}

// PlanResult is the output of Plan(). It includes the saved plan and any
// non-blocking warnings. If Errors is non-empty the plan status is "blocked".
type PlanResult struct {
	Plan     MobilityPlan
	Warnings []ValidationWarning
	Errors   []ValidationError
}

// Plan generates, validates, and stores a MobilityPlan.
// It does not modify any infrastructure.
func (p *Planner) Plan(req PlanRequest) (PlanResult, error) {
	var result PlanResult

	// 1. Check for existing mobility lock on the source VPC.
	locks, err := p.store.ListLocks(req.SourceVPCID)
	if err != nil {
		return result, fmt.Errorf("check locks: %w", err)
	}
	if len(locks) > 0 {
		result.Errors = append(result.Errors, ValidationError{
			Code:    CodeActiveLock,
			Message: fmt.Sprintf("VPC %q already has an active mobility lock", req.SourceVPCID),
		})
	}

	// 2. Build or use injected inventory.
	inv := req.Inventory
	if inv == nil {
		inv = buildStubInventory(req.SourceVPCID)
	}

	// 3. Run compatibility checks.
	warnings, errs := checkCompatibility(req, inv)
	result.Warnings = append(result.Warnings, warnings...)
	result.Errors = append(result.Errors, errs...)

	// 4. Generate ordered steps.
	steps := generateSteps(req.Operation, req.Options)

	// 5. Marshal plan/inventory/steps/warnings/errors to JSON.
	invJSON, _ := json.Marshal(inv)
	stepsJSON, _ := json.Marshal(steps)
	warnJSON, _ := json.Marshal(result.Warnings)
	errJSON, _ := json.Marshal(result.Errors)
	optJSON, _ := json.Marshal(req.Options)
	includeJSON, _ := json.Marshal(req.Options.IncludeResources)
	excludeJSON, _ := json.Marshal(req.Options.ExcludeResources)

	status := PlanStatusValidated
	if len(result.Errors) > 0 {
		status = PlanStatusBlocked
	}

	plan := MobilityPlan{
		OrgID:          req.OrgID,
		AccountID:      req.AccountID,
		ProjectID:      req.ProjectID,
		SourceVPCID:    req.SourceVPCID,
		Operation:      req.Operation,
		Strategy:       string(req.Options.Strategy),
		TargetRealmID:  req.TargetRealmID,
		TargetRegionID: req.TargetRegionID,
		TargetZoneID:   req.TargetZoneID,
		Status:         status,
		IncludeJSON:    string(includeJSON),
		ExcludeJSON:    string(excludeJSON),
		OptionsJSON:    string(optJSON),
		InventoryJSON:  string(invJSON),
		PlanJSON:       string(stepsJSON),
		WarningsJSON:   string(warnJSON),
		ErrorsJSON:     string(errJSON),
		CreatedBy:      req.CreatedBy,
	}

	saved, err := p.store.CreatePlan(plan)
	if err != nil {
		return result, fmt.Errorf("save plan: %w", err)
	}
	result.Plan = saved
	return result, nil
}

// ApprovePlan marks a validated plan as approved so it can be executed.
// Blocked plans cannot be approved.
func (p *Planner) ApprovePlan(planID string) error {
	plan, err := p.store.GetPlan(planID)
	if err != nil {
		return err
	}
	if plan.Status != PlanStatusValidated {
		return fmt.Errorf("plan %q status is %q; only validated plans can be approved", planID, plan.Status)
	}
	return p.store.UpdatePlanStatus(planID, PlanStatusApproved)
}

// StartJob creates a MobilityJob from an approved plan.
// The plan is transitioned to "executing"; the job is queued.
func (p *Planner) StartJob(planID, createdBy string) (MobilityJob, error) {
	plan, err := p.store.GetPlan(planID)
	if err != nil {
		return MobilityJob{}, err
	}
	if plan.Status != PlanStatusApproved {
		return MobilityJob{}, fmt.Errorf("plan %q must be approved before execution (status=%q)", planID, plan.Status)
	}

	job := MobilityJob{
		PlanID:      plan.ID,
		OrgID:       plan.OrgID,
		AccountID:   plan.AccountID,
		SourceVPCID: plan.SourceVPCID,
		Operation:   plan.Operation,
		Status:      JobStatusQueued,
		CreatedBy:   createdBy,
	}

	// Acquire mobility lock on the source VPC.
	if err := p.store.AcquireLock(VPCLock{
		OrgID:     plan.OrgID,
		AccountID: plan.AccountID,
		VPCID:     plan.SourceVPCID,
		LockType:  LockTypeMove,
		Reason:    fmt.Sprintf("mobility job for plan %s", plan.ID),
		ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
		CreatedBy: createdBy,
	}); err != nil {
		return MobilityJob{}, fmt.Errorf("acquire lock: %w", err)
	}

	_ = p.store.UpdatePlanStatus(planID, PlanStatusExecuting)

	return p.store.CreateJob(job)
}

// CancelJob marks a queued or running job as cancelled and releases its lock.
func (p *Planner) CancelJob(jobID string) error {
	job, err := p.store.GetJob(jobID)
	if err != nil {
		return err
	}
	if job.Status != JobStatusQueued && job.Status != JobStatusRunning &&
		job.Status != JobStatusWaitingApproval && job.Status != JobStatusWaitingCutover {
		return fmt.Errorf("job %q cannot be cancelled in state %q", jobID, job.Status)
	}
	_ = p.store.UpdateJobStatus(jobID, JobStatusCancelled, job.CurrentStep, job.ProgressPercent)
	_ = p.store.ReleaseLock(job.SourceVPCID, LockTypeMove)
	return nil
}

// DryRun generates a plan without persisting it. Returns the step list and
// any compatibility warnings/errors. Useful for --dry-run CLI output.
func (p *Planner) DryRun(req PlanRequest) ([]string, []ValidationWarning, []ValidationError) {
	inv := req.Inventory
	if inv == nil {
		inv = buildStubInventory(req.SourceVPCID)
	}
	warnings, errs := checkCompatibility(req, inv)
	steps := generateSteps(req.Operation, req.Options)
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s
	}
	return names, warnings, errs
}

// ---- compatibility check ----------------------------------------------------

func checkCompatibility(req PlanRequest, inv *VPCInventory) ([]ValidationWarning, []ValidationError) {
	var warnings []ValidationWarning
	var errs []ValidationError

	if req.SourceVPCID == "" {
		errs = append(errs, ValidationError{Code: CodeSourceNotFound, Message: "source VPC ID is required"})
	}

	if req.Operation == OperationMove || req.Operation == OperationRelocate {
		if req.TargetRegionID == "" {
			errs = append(errs, ValidationError{Code: CodeDestRegionNotFound, Message: "destination region required for move"})
		}
		if req.TargetZoneID == "" {
			errs = append(errs, ValidationError{Code: CodeDestZoneNotFound, Message: "destination zone required for move"})
		}
	}

	if req.Operation == OperationDelete && inv != nil && inv.InstanceCount > 0 &&
		req.Options.CopyMode != CopyModeFull {
		errs = append(errs, ValidationError{
			Code:    CodeDeleteBlocked,
			Message: fmt.Sprintf("VPC has %d running instances; use cascade delete mode", inv.InstanceCount),
		})
	}

	// Warn about public IP portability for cross-region moves.
	if req.Operation == OperationMove && req.TargetRegionID != "" {
		warnings = append(warnings, ValidationWarning{
			Code:    CodePublicIPNotPortable,
			Message: "Public IPs may not be portable to the target region.",
			Impact:  "New public IPs will be assigned if portability fails.",
		})
	}

	return warnings, errs
}

// ---- step generation --------------------------------------------------------

func generateSteps(op Operation, opts PlanOptions) []string {
	switch op {
	case OperationCopy:
		return copySteps(opts.CopyMode)
	case OperationMove:
		return moveSteps(opts.Strategy)
	case OperationDelete:
		return deleteSteps()
	case OperationFailover:
		return failoverSteps()
	default:
		return []string{"unknown-operation"}
	}
}

func copySteps(mode CopyMode) []string {
	base := []string{
		"lock-source-vpc",
		"inventory-source-vpc",
		"validate-destination-capacity",
		"create-destination-vpc",
		"copy-subnets",
		"copy-routes",
		"copy-firewall-rules",
		"copy-security-groups",
	}
	switch mode {
	case CopyModeTopologyOnly:
		return append(base, "release-locks")
	case CopyModeTopologyAndCompute:
		return append(base,
			"create-destination-instances-stopped",
			"release-locks",
		)
	case CopyModeTopologyComputeStorage:
		return append(base,
			"snapshot-volumes",
			"copy-snapshots",
			"create-destination-volumes",
			"create-destination-instances-stopped",
			"attach-volumes",
			"release-locks",
		)
	default: // full
		return append(base,
			"snapshot-volumes",
			"copy-snapshots",
			"create-destination-volumes",
			"create-destination-instances-stopped",
			"attach-volumes",
			"copy-load-balancers",
			"copy-ingress-rules",
			"copy-dns-records-disabled",
			"copy-waf-rules",
			"run-health-checks",
			"release-locks",
		)
	}
}

func moveSteps(strategy Strategy) []string {
	base := copySteps(CopyModeFull)
	// Remove final "release-locks" — we need to keep source locked until cutover.
	if len(base) > 0 && base[len(base)-1] == "release-locks" {
		base = base[:len(base)-1]
	}
	switch strategy {
	case StrategyEmergencyRestore:
		return append(base,
			"start-destination-instances",
			"cutover-load-balancers",
			"cutover-dns",
			"mark-source-failed",
			"release-locks",
		)
	default: // planned-cutover
		return append(base,
			"run-health-checks",
			"freeze-source",
			"final-sync",
			"start-destination-instances",
			"cutover-load-balancers",
			"cutover-dns",
			"verify-cutover",
			"mark-source-retired",
			"release-locks",
		)
	}
}

func deleteSteps() []string {
	return []string{
		"lock-source-vpc",
		"check-delete-policy",
		"disable-ingress",
		"detach-dns-records",
		"delete-load-balancers",
		"stop-instances",
		"snapshot-volumes",
		"delete-instances",
		"delete-volumes",
		"delete-firewall-rules",
		"delete-routes",
		"delete-subnets",
		"delete-vpc",
		"release-locks",
	}
}

func failoverSteps() []string {
	return []string{
		"validate-replica",
		"lock-source-vpc",
		"cutover-load-balancers",
		"cutover-dns",
		"activate-replica-instances",
		"verify-cutover",
		"release-locks",
	}
}

// buildStubInventory returns a minimal VPCInventory for planning purposes
// when no live inventory source is available.
func buildStubInventory(vpcID string) *VPCInventory {
	return &VPCInventory{VPCID: vpcID}
}
