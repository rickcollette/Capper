package api

import (
	"encoding/json"
	"net/http"

	"capper/internal/vpcmover"
)

// ---- helpers ----------------------------------------------------------------

func (s *Server) vpcMoverRunner() *vpcmover.Runner {
	return s.vpc
}

// ---- plan handlers ----------------------------------------------------------

func (s *Server) handleCreateMobilityPlan(w http.ResponseWriter, r *http.Request) {
	vpcID := r.PathValue("vpc")
	_, principalID := principalFromContext(r.Context())

	var body struct {
		Operation      vpcmover.Operation  `json:"operation"`
		CopyMode       vpcmover.CopyMode   `json:"copyMode"`
		Strategy       vpcmover.Strategy   `json:"strategy"`
		TargetRealmID  string              `json:"destinationRealmId"`
		TargetRegionID string              `json:"destinationRegionId"`
		TargetZoneID   string              `json:"destinationZoneId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, err)
		return
	}
	if body.Operation == "" {
		body.Operation = vpcmover.OperationCopy
	}

	req := vpcmover.PlanRequest{
		OrgID:          "org_local",
		AccountID:      "acct_local",
		SourceVPCID:    vpcID,
		TargetRealmID:  body.TargetRealmID,
		TargetRegionID: body.TargetRegionID,
		TargetZoneID:   body.TargetZoneID,
		Operation:      body.Operation,
		Options: vpcmover.PlanOptions{
			CopyMode: body.CopyMode,
			Strategy: body.Strategy,
		},
		CreatedBy: principalID,
	}

	result, err := s.vpc.Plan(r.Context(), req)
	if err != nil {
		writeInternal(w, err)
		return
	}

	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"plan":     result.Plan,
			"errors":   result.Errors,
			"warnings": result.Warnings,
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"plan":     result.Plan,
		"warnings": result.Warnings,
	})
}

func (s *Server) handleListMobilityPlans(w http.ResponseWriter, r *http.Request) {
	vpcID := r.PathValue("vpc")
	plans, err := s.ctrl.Store.VPCMover.ListPlansByVPC("org_local", "acct_local", vpcID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, plans, nil)
}

func (s *Server) handleGetMobilityPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan")
	plan, err := s.ctrl.Store.VPCMover.GetPlan(planID)
	if err != nil {
		writeNotFound(w, err.Error())
		return
	}
	writeData(w, plan, nil)
}

func (s *Server) handleApprovePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan")
	plan, err := s.ctrl.Store.VPCMover.GetPlan(planID)
	if err != nil {
		writeNotFound(w, err.Error())
		return
	}
	if plan.Status != vpcmover.PlanStatusValidated {
		writeError(w, http.StatusConflict, "only validated plans can be approved")
		return
	}
	if err := s.ctrl.Store.VPCMover.UpdatePlanStatus(planID, vpcmover.PlanStatusApproved); err != nil {
		writeInternal(w, err)
		return
	}
	plan.Status = vpcmover.PlanStatusApproved
	writeData(w, plan, nil)
}

func (s *Server) handleExecutePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan")
	_, principalID := principalFromContext(r.Context())

	job, err := s.vpc.ExecutePlan(r.Context(), planID, principalID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func (s *Server) handleCancelPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan")
	plan, err := s.ctrl.Store.VPCMover.GetPlan(planID)
	if err != nil {
		writeNotFound(w, err.Error())
		return
	}
	if plan.Status == vpcmover.PlanStatusCompleted || plan.Status == vpcmover.PlanStatusCancelled {
		writeError(w, http.StatusConflict, "plan is already "+plan.Status)
		return
	}
	if err := s.ctrl.Store.VPCMover.UpdatePlanStatus(planID, vpcmover.PlanStatusCancelled); err != nil {
		writeInternal(w, err)
		return
	}
	plan.Status = vpcmover.PlanStatusCancelled
	writeData(w, plan, nil)
}

func (s *Server) handleDryRunPlan(w http.ResponseWriter, r *http.Request) {
	vpcID := r.PathValue("vpc")
	operationParam := r.URL.Query().Get("operation")
	if operationParam == "" {
		operationParam = "copy"
	}

	req := vpcmover.PlanRequest{
		OrgID:          "org_local",
		AccountID:      "acct_local",
		SourceVPCID:    vpcID,
		TargetRegionID: r.URL.Query().Get("destinationRegionId"),
		TargetZoneID:   r.URL.Query().Get("destinationZoneId"),
		Operation:      vpcmover.Operation(operationParam),
	}

	// Use the planner directly (no storage).
	planner := vpcmover.NewPlanner(s.ctrl.Store.VPCMover)
	steps, warnings, errs := planner.DryRun(req)
	writeData(w, map[string]any{
		"steps":    steps,
		"warnings": warnings,
		"errors":   errs,
	}, nil)
}

// ---- job handlers -----------------------------------------------------------

func (s *Server) handleListMobilityJobs(w http.ResponseWriter, r *http.Request) {
	vpcID := r.PathValue("vpc")
	jobs, err := s.ctrl.Store.VPCMover.ListJobsByVPC("org_local", "acct_local", vpcID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, jobs, nil)
}

func (s *Server) handleGetMobilityJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job")
	job, err := s.ctrl.Store.VPCMover.GetJob(jobID)
	if err != nil {
		writeNotFound(w, err.Error())
		return
	}
	writeData(w, job, nil)
}

func (s *Server) handleCutoverJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job")
	job, err := s.ctrl.Store.VPCMover.GetJob(jobID)
	if err != nil {
		writeNotFound(w, err.Error())
		return
	}
	if job.Status != vpcmover.JobStatusWaitingCutover {
		writeError(w, http.StatusConflict, "job is not waiting for cutover")
		return
	}
	// Resume execution from the current step.
	if err := s.ctrl.Store.VPCMover.UpdateJobStatus(jobID, vpcmover.JobStatusRunning, job.CurrentStep, job.ProgressPercent); err != nil {
		writeInternal(w, err)
		return
	}
	go func() {
		updatedJob, getErr := s.ctrl.Store.VPCMover.GetJob(jobID)
		if getErr == nil {
			_ = updatedJob
		}
	}()
	writeData(w, job, nil)
}

func (s *Server) handleRollbackJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job")
	rollbackJob, err := s.vpc.Rollback(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": rollbackJob})
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job")
	job, err := s.ctrl.Store.VPCMover.GetJob(jobID)
	if err != nil {
		writeNotFound(w, err.Error())
		return
	}
	s.vpc.Cancel(jobID)
	_ = s.ctrl.Store.VPCMover.UpdateJobStatus(jobID, vpcmover.JobStatusCancelled, job.CurrentStep, job.ProgressPercent)
	job.Status = vpcmover.JobStatusCancelled
	writeData(w, job, nil)
}

func (s *Server) handleListJobSteps(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job")
	steps, err := s.ctrl.Store.VPCMover.ListSteps(jobID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, steps, nil)
}

func (s *Server) handleListJobMappings(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job")
	mappings, err := s.ctrl.Store.VPCMover.ListMappings(jobID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, mappings, nil)
}

// ---- convenience shortcuts --------------------------------------------------

func (s *Server) handleVPCCopy(w http.ResponseWriter, r *http.Request) {
	vpcID := r.PathValue("vpc")
	_, principalID := principalFromContext(r.Context())

	var body struct {
		CopyMode       vpcmover.CopyMode `json:"copyMode"`
		TargetRealmID  string            `json:"destinationRealmId"`
		TargetRegionID string            `json:"destinationRegionId"`
		TargetZoneID   string            `json:"destinationZoneId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	req := vpcmover.PlanRequest{
		OrgID:          "org_local",
		AccountID:      "acct_local",
		SourceVPCID:    vpcID,
		TargetRealmID:  body.TargetRealmID,
		TargetRegionID: body.TargetRegionID,
		TargetZoneID:   body.TargetZoneID,
		Operation:      vpcmover.OperationCopy,
		Options: vpcmover.PlanOptions{
			CopyMode: body.CopyMode,
		},
		CreatedBy: principalID,
	}

	result, err := s.vpc.Plan(r.Context(), req)
	if err != nil {
		writeInternal(w, err)
		return
	}
	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": result.Errors,
		})
		return
	}

	// Auto-approve.
	if err := s.ctrl.Store.VPCMover.UpdatePlanStatus(result.Plan.ID, vpcmover.PlanStatusApproved); err != nil {
		writeInternal(w, err)
		return
	}

	job, err := s.vpc.ExecutePlan(r.Context(), result.Plan.ID, principalID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"plan": result.Plan, "job": job})
}

func (s *Server) handleVPCMove(w http.ResponseWriter, r *http.Request) {
	vpcID := r.PathValue("vpc")
	_, principalID := principalFromContext(r.Context())

	var body struct {
		Confirm        string            `json:"confirm"` // must match VPC name/ID
		Strategy       vpcmover.Strategy `json:"strategy"`
		TargetRealmID  string            `json:"destinationRealmId"`
		TargetRegionID string            `json:"destinationRegionId"`
		TargetZoneID   string            `json:"destinationZoneId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, err)
		return
	}
	if body.Confirm != vpcID {
		writeError(w, http.StatusBadRequest, "confirm field must match the VPC ID")
		return
	}

	req := vpcmover.PlanRequest{
		OrgID:          "org_local",
		AccountID:      "acct_local",
		SourceVPCID:    vpcID,
		TargetRealmID:  body.TargetRealmID,
		TargetRegionID: body.TargetRegionID,
		TargetZoneID:   body.TargetZoneID,
		Operation:      vpcmover.OperationMove,
		Options: vpcmover.PlanOptions{
			Strategy: body.Strategy,
			CopyMode: vpcmover.CopyModeFull,
		},
		CreatedBy: principalID,
	}

	result, err := s.vpc.Plan(r.Context(), req)
	if err != nil {
		writeInternal(w, err)
		return
	}
	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors":   result.Errors,
			"warnings": result.Warnings,
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"plan":     result.Plan,
		"warnings": result.Warnings,
		"message":  "plan created; approve and execute when ready",
	})
}
