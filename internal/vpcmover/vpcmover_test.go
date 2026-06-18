package vpcmover_test

import (
	"database/sql"
	"testing"

	"capper/internal/vpcmover"

	_ "modernc.org/sqlite"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := vpcmover.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

func TestCreateAndGetPlan(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	p, err := s.CreatePlan(vpcmover.MobilityPlan{
		OrgID: "org_1", AccountID: "acct_1",
		SourceVPCID: "vpc_prod", Operation: vpcmover.OperationMove,
		Status: vpcmover.PlanStatusValidated,
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	got, err := s.GetPlan(p.ID)
	if err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	if got.SourceVPCID != "vpc_prod" {
		t.Errorf("SourceVPCID = %q want vpc_prod", got.SourceVPCID)
	}
}

func TestListPlansByVPC(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	for i := 0; i < 3; i++ {
		_, _ = s.CreatePlan(vpcmover.MobilityPlan{
			OrgID: "org_1", AccountID: "acct_1",
			SourceVPCID: "vpc_prod", Operation: vpcmover.OperationCopy,
		})
	}
	plans, err := s.ListPlansByVPC("org_1", "acct_1", "vpc_prod")
	if err != nil {
		t.Fatalf("ListPlansByVPC: %v", err)
	}
	if len(plans) != 3 {
		t.Errorf("expected 3 plans, got %d", len(plans))
	}
}

func TestUpdatePlanStatus(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	p, _ := s.CreatePlan(vpcmover.MobilityPlan{
		OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationMove,
	})
	if err := s.UpdatePlanStatus(p.ID, vpcmover.PlanStatusApproved); err != nil {
		t.Fatalf("UpdatePlanStatus: %v", err)
	}
	got, _ := s.GetPlan(p.ID)
	if got.Status != vpcmover.PlanStatusApproved {
		t.Errorf("status = %q want approved", got.Status)
	}
}

func TestCreateAndGetJob(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	p, _ := s.CreatePlan(vpcmover.MobilityPlan{
		OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationMove,
	})
	j, err := s.CreateJob(vpcmover.MobilityJob{
		PlanID: p.ID, OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationMove,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	got, err := s.GetJob(j.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Status != vpcmover.JobStatusQueued {
		t.Errorf("status = %q want queued", got.Status)
	}
}

func TestStepLifecycle(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	p, _ := s.CreatePlan(vpcmover.MobilityPlan{
		OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationCopy,
	})
	j, _ := s.CreateJob(vpcmover.MobilityJob{
		PlanID: p.ID, OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationCopy,
	})
	step, err := s.CreateStep(vpcmover.MobilityStep{
		JobID: j.ID, StepOrder: 1, Name: "lock-source-vpc",
	})
	if err != nil {
		t.Fatalf("CreateStep: %v", err)
	}
	if err := s.UpdateStepStatus(step.ID, vpcmover.StepStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateStepStatus: %v", err)
	}
	steps, err := s.ListSteps(j.ID)
	if err != nil {
		t.Fatalf("ListSteps: %v", err)
	}
	if len(steps) != 1 || steps[0].Status != vpcmover.StepStatusCompleted {
		t.Errorf("step status unexpected: %v", steps)
	}
}

func TestLockAndRelease(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	err := s.AcquireLock(vpcmover.VPCLock{
		OrgID: "o1", AccountID: "a1",
		VPCID: "vpc_1", LockType: vpcmover.LockTypeMove,
		Reason: "test", JobID: "job_1",
	})
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	// Duplicate lock must fail.
	err = s.AcquireLock(vpcmover.VPCLock{
		OrgID: "o1", AccountID: "a1",
		VPCID: "vpc_1", LockType: vpcmover.LockTypeMove,
		Reason: "dupe",
	})
	if err == nil {
		t.Fatal("expected duplicate lock to fail")
	}

	if err := s.ReleaseLock("vpc_1", vpcmover.LockTypeMove); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	// Should be able to re-acquire after release.
	if err := s.AcquireLock(vpcmover.VPCLock{
		OrgID: "o1", AccountID: "a1",
		VPCID: "vpc_1", LockType: vpcmover.LockTypeMove,
	}); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
}

func TestResourceMappings(t *testing.T) {
	s := vpcmover.NewStore(openDB(t))
	p, _ := s.CreatePlan(vpcmover.MobilityPlan{
		OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationMove,
	})
	j, _ := s.CreateJob(vpcmover.MobilityJob{
		PlanID: p.ID, OrgID: "o1", AccountID: "a1",
		SourceVPCID: "vpc_1", Operation: vpcmover.OperationMove,
	})
	for _, pair := range [][2]string{
		{"subnet_a", "subnet_b"},
		{"sg_1", "sg_2"},
		{"lb_1", "lb_2"},
	} {
		_ = s.RecordMapping(vpcmover.ResourceMapping{
			JobID: j.ID, OrgID: "o1", AccountID: "a1",
			SourceResourceType: "subnet", SourceResourceID: pair[0],
			DestResourceType: "subnet", DestResourceID: pair[1],
		})
	}
	mappings, err := s.ListMappings(j.ID)
	if err != nil {
		t.Fatalf("ListMappings: %v", err)
	}
	if len(mappings) != 3 {
		t.Errorf("expected 3 mappings, got %d", len(mappings))
	}
}

// ---- planner tests ----------------------------------------------------------

func TestPlannerCopyTopologyOnly(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	result, err := planner.Plan(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation: vpcmover.OperationCopy,
		Options:   vpcmover.PlanOptions{CopyMode: vpcmover.CopyModeTopologyOnly},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if result.Plan.Status != vpcmover.PlanStatusValidated {
		t.Errorf("status = %q want validated", result.Plan.Status)
	}
}

func TestPlannerMoveRequiresRegionAndZone(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	result, err := planner.Plan(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation: vpcmover.OperationMove,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if result.Plan.Status != vpcmover.PlanStatusBlocked {
		t.Errorf("status = %q want blocked (missing region/zone)", result.Plan.Status)
	}
	if len(result.Errors) == 0 {
		t.Error("expected validation errors for missing region/zone")
	}
}

func TestPlannerMoveWarnsPublicIP(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	result, err := planner.Plan(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation:      vpcmover.OperationMove,
		TargetRegionID: "region_use2",
		TargetZoneID:   "zone_use2a",
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	found := false
	for _, w := range result.Warnings {
		if w.Code == vpcmover.CodePublicIPNotPortable {
			found = true
		}
	}
	if !found {
		t.Error("expected public IP portability warning for cross-region move")
	}
}

func TestPlannerActiveLockBlocksPlan(t *testing.T) {
	db := openDB(t)
	s := vpcmover.NewStore(db)
	_ = s.AcquireLock(vpcmover.VPCLock{
		OrgID: "o1", AccountID: "a1",
		VPCID: "vpc_1", LockType: vpcmover.LockTypeMove, JobID: "job_x",
	})
	planner := vpcmover.NewPlanner(s)
	result, _ := planner.Plan(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation: vpcmover.OperationCopy,
	})
	if result.Plan.Status != vpcmover.PlanStatusBlocked {
		t.Errorf("expected blocked due to active lock, got %q", result.Plan.Status)
	}
}

func TestPlannerApproveAndStartJob(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	result, err := planner.Plan(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation: vpcmover.OperationCopy,
		Options:   vpcmover.PlanOptions{CopyMode: vpcmover.CopyModeTopologyOnly},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := planner.ApprovePlan(result.Plan.ID); err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}
	job, err := planner.StartJob(result.Plan.ID, "user_1")
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	if job.Status != vpcmover.JobStatusQueued {
		t.Errorf("job status = %q want queued", job.Status)
	}
	if job.SourceVPCID != "vpc_1" {
		t.Errorf("SourceVPCID = %q want vpc_1", job.SourceVPCID)
	}
}

func TestPlannerCannotApproveBlockedPlan(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	result, _ := planner.Plan(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation: vpcmover.OperationMove, // missing region/zone → blocked
	})
	if err := planner.ApprovePlan(result.Plan.ID); err == nil {
		t.Fatal("expected error approving blocked plan")
	}
}

func TestDryRunReturnsStepsOnly(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	steps, warnings, errs := planner.DryRun(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation:      vpcmover.OperationMove,
		TargetRegionID: "region_use2",
		TargetZoneID:   "zone_use2a",
	})
	if len(steps) == 0 {
		t.Error("expected non-empty step list from DryRun")
	}
	if len(errs) != 0 {
		t.Errorf("unexpected errors in dry-run: %v", errs)
	}
	_ = warnings
	_ = steps
}

func TestDeleteStepsOrdering(t *testing.T) {
	db := openDB(t)
	planner := vpcmover.NewPlanner(vpcmover.NewStore(db))
	steps, _, _ := planner.DryRun(vpcmover.PlanRequest{
		OrgID: "o1", AccountID: "a1", SourceVPCID: "vpc_1",
		Operation: vpcmover.OperationDelete,
	})
	// Verify "lock-source-vpc" is first and "delete-vpc" comes before "release-locks".
	if len(steps) < 3 || steps[0] != "lock-source-vpc" {
		t.Errorf("expected lock-source-vpc first, got %v", steps)
	}
	deleteIdx, releaseIdx := -1, -1
	for i, s := range steps {
		if s == "delete-vpc" {
			deleteIdx = i
		}
		if s == "release-locks" {
			releaseIdx = i
		}
	}
	if deleteIdx < 0 || releaseIdx < 0 {
		t.Fatalf("delete-vpc or release-locks missing from steps: %v", steps)
	}
	if deleteIdx > releaseIdx {
		t.Errorf("delete-vpc must come before release-locks")
	}
}
