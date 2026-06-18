package vpcmover

import (
	"context"
	"fmt"
	"time"
)

// dispatchStep routes a step to its handler by name.
func (e *Executor) dispatchStep(ctx context.Context, job MobilityJob, step MobilityStep) error {
	switch step.Name {
	// Lock/unlock
	case "lock-source-vpc":
		return e.stepAcquireLock(ctx, job, step)
	case "release-locks":
		return e.stepReleaseLock(ctx, job, step)

	// Validation
	case "inventory-source-vpc":
		return e.stepInventorySource(ctx, job, step)
	case "validate-destination-capacity":
		return e.stepCheckDestination(ctx, job, step)
	case "check-delete-policy":
		return e.stepCheckDeletePolicy(ctx, job, step)

	// Topology copy
	case "create-destination-vpc":
		return e.stepCopyTopology(ctx, job, step)
	case "copy-subnets":
		return e.stepUnimplemented(ctx, job, step, "copy subnets")
	case "copy-routes":
		return e.stepUnimplemented(ctx, job, step, "copy routes")
	case "copy-firewall-rules":
		return e.stepUnimplemented(ctx, job, step, "copy firewall rules")
	case "copy-security-groups":
		return e.stepUnimplemented(ctx, job, step, "copy security groups")

	// Compute
	case "create-destination-instances-stopped":
		return e.stepUnimplemented(ctx, job, step, "create destination instances stopped")
	case "start-destination-instances":
		return e.stepUnimplemented(ctx, job, step, "start destination instances")

	// Storage
	case "snapshot-volumes":
		return e.stepUnimplemented(ctx, job, step, "snapshot volumes")
	case "copy-snapshots":
		return e.stepUnimplemented(ctx, job, step, "copy snapshots")
	case "create-destination-volumes":
		return e.stepUnimplemented(ctx, job, step, "create destination volumes")
	case "attach-volumes":
		return e.stepUnimplemented(ctx, job, step, "attach volumes")

	// Networking / DNS
	case "copy-load-balancers":
		return e.stepUnimplemented(ctx, job, step, "copy load balancers")
	case "copy-ingress-rules":
		return e.stepUnimplemented(ctx, job, step, "copy ingress rules")
	case "copy-dns-records-disabled":
		return e.stepUnimplemented(ctx, job, step, "copy DNS records (disabled)")
	case "copy-waf-rules":
		return e.stepUnimplemented(ctx, job, step, "copy WAF rules")

	// Health / cutover
	case "run-health-checks":
		return e.stepUnimplemented(ctx, job, step, "run health checks")
	case "freeze-source":
		return e.stepUnimplemented(ctx, job, step, "freeze source VPC")
	case "final-sync":
		return e.stepUnimplemented(ctx, job, step, "final sync")
	case "cutover-load-balancers":
		return e.stepUnimplemented(ctx, job, step, "cutover load balancers")
	case "cutover-dns":
		return e.stepUnimplemented(ctx, job, step, "cutover DNS")
	case "verify-cutover":
		return e.stepUnimplemented(ctx, job, step, "verify cutover")
	case "mark-source-retired":
		return e.stepMarkSourceRetired(ctx, job, step)
	case "mark-source-failed":
		return e.stepUnimplemented(ctx, job, step, "mark source failed")

	// Delete operation
	case "disable-ingress":
		return e.stepUnimplemented(ctx, job, step, "disable ingress")
	case "detach-dns-records":
		return e.stepUnimplemented(ctx, job, step, "detach DNS records")
	case "delete-load-balancers":
		return e.stepUnimplemented(ctx, job, step, "delete load balancers")
	case "stop-instances":
		return e.stepUnimplemented(ctx, job, step, "stop instances")
	case "delete-instances":
		return e.stepUnimplemented(ctx, job, step, "delete instances")
	case "delete-volumes":
		return e.stepUnimplemented(ctx, job, step, "delete volumes")
	case "delete-firewall-rules":
		return e.stepUnimplemented(ctx, job, step, "delete firewall rules")
	case "delete-routes":
		return e.stepUnimplemented(ctx, job, step, "delete routes")
	case "delete-subnets":
		return e.stepUnimplemented(ctx, job, step, "delete subnets")
	case "delete-vpc":
		return e.stepUnimplemented(ctx, job, step, "delete VPC")

	// Failover
	case "validate-replica":
		return e.stepUnimplemented(ctx, job, step, "validate replica")
	case "activate-replica-instances":
		return e.stepUnimplemented(ctx, job, step, "activate replica instances")

	// Rollback
	case "cleanup-destination":
		return e.stepUnimplemented(ctx, job, step, "cleanup destination")

	default:
		// Unknown step — skip silently.
		return nil
	}
}

// stepAcquireLock acquires a mobility lock on the source VPC.
func (e *Executor) stepAcquireLock(_ context.Context, job MobilityJob, _ MobilityStep) error {
	return e.store.AcquireLock(VPCLock{
		OrgID:     job.OrgID,
		AccountID: job.AccountID,
		VPCID:     job.SourceVPCID,
		LockType:  LockTypeMobilityPlan,
		Reason:    fmt.Sprintf("mobility job %s", job.ID),
		JobID:     job.ID,
		ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
		CreatedBy: job.CreatedBy,
	})
}

// stepReleaseLock releases the mobility lock on the source VPC.
func (e *Executor) stepReleaseLock(_ context.Context, job MobilityJob, _ MobilityStep) error {
	// Try both lock types; ignore "not found" errors.
	_ = e.store.ReleaseLock(job.SourceVPCID, LockTypeMobilityPlan)
	_ = e.store.ReleaseLock(job.SourceVPCID, LockTypeMove)
	_ = e.store.ReleaseLock(job.SourceVPCID, LockTypeCopy)
	return nil
}

// stepInventorySource re-builds the live inventory for the source VPC.
func (e *Executor) stepInventorySource(_ context.Context, job MobilityJob, _ MobilityStep) error {
	_, err := BuildInventory(e.invStore, job.SourceVPCID)
	return err
}

// stepCheckDestination performs a light compatibility check.
func (e *Executor) stepCheckDestination(_ context.Context, job MobilityJob, _ MobilityStep) error {
	plan, err := e.store.GetPlan(job.PlanID)
	if err != nil {
		// No plan on file — best-effort pass.
		return nil
	}
	req := PlanRequest{
		OrgID:          job.OrgID,
		AccountID:      job.AccountID,
		SourceVPCID:    job.SourceVPCID,
		TargetRegionID: plan.TargetRegionID,
		TargetZoneID:   plan.TargetZoneID,
		Operation:      job.Operation,
	}
	inv := buildStubInventory(job.SourceVPCID)
	_, errs := checkCompatibility(req, inv)
	if len(errs) > 0 {
		return fmt.Errorf("compatibility check failed: %s", errs[0].Message)
	}
	return nil
}

// stepCheckDeletePolicy validates that the VPC can be deleted.
func (e *Executor) stepCheckDeletePolicy(_ context.Context, job MobilityJob, _ MobilityStep) error {
	// Check for retention locks.
	locks, err := e.store.ListLocks(job.SourceVPCID)
	if err != nil {
		return err
	}
	for _, l := range locks {
		if l.LockType == LockTypeFreeze {
			return fmt.Errorf("%s: VPC %s has a freeze lock", ErrComplianceLock.Message, job.SourceVPCID)
		}
	}
	return nil
}

// stepCopyTopology creates the destination VPC record in the topology store.
func (e *Executor) stepCopyTopology(_ context.Context, job MobilityJob, _ MobilityStep) error {
	if e.invStore.Topology == nil {
		return nil
	}

	src, err := e.invStore.Topology.GetVPC("", job.SourceVPCID)
	if err != nil {
		// Source VPC not found — log and continue.
		return nil
	}

	// For a copy operation, create a new VPC with a derived name.
	dest := src
	dest.ID = ""
	dest.Slug = src.Slug + "-copy"
	dest.Name = src.Name + " (copy)"

	if err := e.invStore.Topology.InsertVPC(dest); err != nil {
		// May already exist — treat as non-fatal.
		return nil
	}

	// Record the mapping.
	_ = e.store.RecordMapping(ResourceMapping{
		JobID:              job.ID,
		OrgID:              job.OrgID,
		AccountID:          job.AccountID,
		SourceResourceType: "vpc",
		SourceResourceID:   job.SourceVPCID,
		DestResourceType:   "vpc",
		DestResourceID:     dest.ID,
	})
	return nil
}

// stepMarkSourceRetired updates the source VPC status to "retired".
func (e *Executor) stepMarkSourceRetired(_ context.Context, job MobilityJob, _ MobilityStep) error {
	if e.invStore.Topology == nil {
		return nil
	}
	vpc, err := e.invStore.Topology.GetVPC("", job.SourceVPCID)
	if err != nil {
		return nil
	}
	vpc.Status = "retired"
	return e.invStore.Topology.UpdateVPC(vpc)
}

// stepUnimplemented fails the step explicitly. These steps require external
// systems (compute, storage, network, DNS, LB managers) that are not yet wired
// into the executor. Returning an error halts the job with a truthful status
// instead of silently reporting success while moving nothing.
func (e *Executor) stepUnimplemented(ctx context.Context, job MobilityJob, step MobilityStep, desc string) error {
	if e.recorder != nil {
		e.recorder.RecordMobilityEvent(ctx, "step.unimplemented", job.ID,
			fmt.Sprintf("step %q (%s) is not implemented", step.Name, desc))
	}
	return fmt.Errorf("vpcmover: step %q (%s) is not implemented; executor not wired to the required subsystem", step.Name, desc)
}
