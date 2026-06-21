package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"capper/internal/types"
)

// handleDeleteResourcePreflight returns a deletion plan without actually deleting.
// Clients use this to see what will be deleted and get a confirmation token.
// Route: POST /api/v1/{resourceType}/{resourceId}:delete-preflight
func (s *Server) handleDeleteResourcePreflight(w http.ResponseWriter, r *http.Request) {
	resourceType := r.PathValue("resourceType")
	resourceID := r.PathValue("resourceId")

	// Generate confirmation token for this deletion
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeInternal(w, fmt.Errorf("failed to generate confirmation token: %w", err))
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Build resource-specific deletion order based on resource type
	deleteOrder := []string{}
	switch resourceType {
	case "instance":
		deleteOrder = []string{
			resourceID + " (stop)",
			resourceID + " (detach-eni)",
			resourceID + " (delete)",
		}
	case "vpc":
		deleteOrder = []string{
			"subnets",
			"network-acls",
			"route-tables",
			"internet-gateways",
			resourceID + " (vpc)",
		}
	case "database":
		deleteOrder = []string{
			resourceID + " (backup)",
			resourceID + " (delete)",
		}
	case "load-balancer":
		deleteOrder = []string{
			"target-groups",
			"listeners",
			resourceID + " (load-balancer)",
		}
	default:
		deleteOrder = []string{resourceID}
	}

	plan := map[string]any{
		"resourceType":         resourceType,
		"resourceId":           resourceID,
		"confirmationToken":    token,
		"requiresConfirmation": true,
		"deleteOrder":          deleteOrder,
		"message": "Use the confirmationToken to proceed with deletion. " +
			"Confirmation requires typing DELETE in uppercase.",
	}

	writeData(w, plan, nil)
}

// handleDeleteResourceConfirm creates a deletion job and starts async deletion.
// Clients must provide the confirmation token and phrase "DELETE" (uppercase).
// Route: POST /api/v1/{resourceType}/{resourceId}:delete-confirm
func (s *Server) handleDeleteResourceConfirm(w http.ResponseWriter, r *http.Request) {
	resourceType := r.PathValue("resourceType")
	resourceID := r.PathValue("resourceId")

	var req struct {
		ConfirmationToken  string `json:"confirmationToken"`
		ConfirmationPhrase string `json:"confirmationPhrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}

	// Validate confirmation phrase (must be exactly "DELETE" in uppercase)
	if req.ConfirmationPhrase != "DELETE" {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("confirmationPhrase must be exactly \"DELETE\" (uppercase); got %q", req.ConfirmationPhrase))
		return
	}

	// TODO: Validate confirmation token (check it was recently issued).
	// For now, accept any non-empty token.
	if req.ConfirmationToken == "" {
		writeError(w, http.StatusBadRequest, "confirmationToken is required")
		return
	}

	// Create deletion job
	jobID := "del-" + generateJobID()
	job := &types.DeletionJob{
		ID:                jobID,
		Status:            "queued",
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		ConfirmationToken: req.ConfirmationToken,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(7 * 24 * time.Hour), // auto-cleanup after 7 days
		Steps: []string{
			"validate", "disconnect", "delete",
		},
	}

	if err := s.ctrl.Store.DeletionJobs.Create(job); err != nil {
		slog.Error("failed to create deletion job", "jobId", jobID, "error", err)
		writeInternal(w, fmt.Errorf("failed to create deletion job: %w", err))
		return
	}

	slog.Info("deletion job created", "jobId", jobID, "resourceType", resourceType, "resourceId", resourceID)

	// Verify job was saved (for debugging)
	_, verifyErr := s.ctrl.Store.DeletionJobs.Get(jobID)
	if verifyErr != nil {
		slog.Warn("created job not immediately retrievable", "jobId", jobID, "error", verifyErr)
	}

	// Start async deletion in background
	go s.asyncDelete(jobID, resourceType, resourceID)

	writeJSON(w, http.StatusAccepted, Envelope{
		Data: map[string]any{
			"jobId":   jobID,
			"status":  "queued",
			"pollUrl": fmt.Sprintf("/api/v1/deletion-jobs/%s", jobID),
		},
	})
}

// handleGetDeletionJob returns the current status of a deletion job.
// Clients poll this endpoint to monitor progress.
// Route: GET /api/v1/deletion-jobs/{jobId}
func (s *Server) handleGetDeletionJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")

	job, err := s.ctrl.Store.DeletionJobs.Get(jobID)
	if err != nil {
		writeNotFound(w, fmt.Sprintf("deletion job %q not found", jobID))
		return
	}

	writeData(w, job, nil)
}

// asyncDelete executes a resource deletion asynchronously.
// This function is called in a goroutine and should handle all cleanup itself.
func (s *Server) asyncDelete(jobID, resourceType, resourceID string) {
	job, err := s.ctrl.Store.DeletionJobs.Get(jobID)
	if err != nil {
		slog.Error("deletion job not found", "jobId", jobID, "error", err)
		return
	}

	// Mark job as started (persist to database)
	if err := s.ctrl.Store.DeletionJobs.UpdateStarted(jobID); err != nil {
		slog.Error("failed to mark job as started", "jobId", jobID, "error", err)
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic during deletion", "jobId", jobID, "panic", r)
			job.Status = "failed"
			s.ctrl.Store.DeletionJobs.AddError(jobID, types.DeletionJobError{
				Step:        "unknown",
				Reason:      fmt.Sprintf("panic: %v", r),
				Recoverable: false,
				Recovery:    "Check logs for details",
			})
			s.ctrl.Store.DeletionJobs.Complete(jobID, false)
		}
	}()

	// Execute resource-specific deletion based on type
	var deleteErr error
	switch resourceType {
	case "instance":
		deleteErr = s.asyncDeleteInstance(jobID, resourceID)
	case "vpc":
		deleteErr = s.asyncDeleteVPC(jobID, resourceID)
	case "load-balancer":
		deleteErr = s.asyncDeleteLoadBalancer(jobID, resourceID)
	case "database":
		deleteErr = s.asyncDeleteDatabase(jobID, resourceID)
	default:
		deleteErr = fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if deleteErr != nil {
		slog.Error("deletion failed", "jobId", jobID, "resourceType", resourceType, "resourceId", resourceID, "error", deleteErr)
		s.ctrl.Store.DeletionJobs.Complete(jobID, false)
	} else {
		slog.Info("deletion completed", "jobId", jobID, "resourceType", resourceType, "resourceId", resourceID)
		s.ctrl.Store.DeletionJobs.Complete(jobID, true)
	}
}

// asyncDeleteInstance deletes an instance and updates job progress.
func (s *Server) asyncDeleteInstance(jobID, instanceID string) error {
	steps := []string{"validate", "disconnect", "remove"}

	// Step 1: Validate
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, "validate", []string{}, steps[1:], 10)
	inst, err := s.ctrl.Store.ResolveInstance(instanceID)
	if err != nil {
		return s.addDeletionError(jobID, "validate", "instance", instanceID, "instance not found", false, "Check instance ID")
	}

	// Step 2: Disconnect from network
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, "disconnect", []string{"validate"}, steps[2:], 40)
	_, _, err = s.ctrl.Instances.Stop(instanceID, 5*time.Second, false)
	if err != nil {
		return s.addDeletionError(jobID, "disconnect", "instance", instanceID, fmt.Sprintf("cannot stop: %v", err), true, "Check instance logs")
	}

	// Step 3: Remove
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, "remove", []string{"validate", "disconnect"}, []string{}, 70)
	if err := s.ctrl.Instances.Remove(instanceID); err != nil {
		return s.addDeletionError(jobID, "remove", "instance", instanceID, fmt.Sprintf("cannot remove: %v", err), true, "Check disk space and permissions")
	}

	_ = s.ctrl.Store.Billing.ReleaseUsage(inst.Labels["project"], "instance", instanceID)
	s.recordDeletionEvent("instance", instanceID, "instance.deleted", nil)

	return nil
}

// asyncDeleteVPC deletes a VPC and its dependent resources.
func (s *Server) asyncDeleteVPC(jobID, vpcID string) error {
	steps := []string{
		"validate",
		"delete-instances",
		"delete-load-balancers",
		"delete-vpc",
	}

	// Step 1: Validate
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[0], []string{}, steps[1:], 10)
	vpc, err := s.ctrl.Store.VPC.GetVPC(vpcID, s.project)
	if err != nil {
		return s.addDeletionError(jobID, steps[0], "vpc", vpcID, "vpc not found", false, "Check VPC ID")
	}

	// Step 2: Delete instances in this VPC
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[1], []string{steps[0]}, steps[2:], 30)
	instances, _ := s.ctrl.Instances.List()
	for _, inst := range instances {
		// Check if instance belongs to this VPC
		if inst.VPCID == vpcID {
			if _, _, err := s.ctrl.Instances.Stop(inst.ID, 5*time.Second, true); err != nil {
				slog.Warn("failed to stop instance during vpc deletion", "instance", inst.ID, "error", err)
			}
			if err := s.ctrl.Instances.Remove(inst.ID); err != nil {
				return s.addDeletionError(jobID, steps[1], "instance", inst.ID, fmt.Sprintf("cannot remove: %v", err), true, "Check instance state")
			}
			slog.Info("deleted instance during vpc deletion", "jobId", jobID, "instance", inst.ID, "vpc", vpcID)
		}
	}

	// Step 3: Delete load balancers attached to this VPC
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[2], []string{steps[0], steps[1]}, steps[3:], 60)
	lbs, _ := s.ctrl.Store.LB.List(s.project)
	for _, lb := range lbs {
		// Check if LB is attached to this VPC
		if lb.VPCID == vpcID {
			if err := s.ctrl.Store.LB.Delete(lb.ID, s.project); err != nil {
				return s.addDeletionError(jobID, steps[2], "load-balancer", lb.ID, fmt.Sprintf("cannot delete: %v", err), true, "Check load balancer state")
			}
			slog.Info("deleted load balancer during vpc deletion", "jobId", jobID, "lb", lb.ID, "vpc", vpcID)
		}
	}

	// Step 4: Delete the VPC (cascades all remaining children via store.DeleteVPC)
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[3], []string{steps[0], steps[1], steps[2]}, []string{}, 90)
	if err := s.ctrl.Store.VPC.DeleteVPC(vpcID, s.project); err != nil {
		return s.addDeletionError(jobID, steps[3], "vpc", vpcID, fmt.Sprintf("cannot delete: %v", err), false, "Check VPC state")
	}

	// Also delete from topology.vpcs table (VPCs are stored in both places)
	if err := s.ctrl.Store.Topology.Store().DeleteVPC(s.project, vpc.Slug); err != nil {
		slog.Warn("failed to delete vpc from topology", "vpc", vpcID, "error", err)
		// Continue anyway; the main vpc deletion succeeded
	}

	s.recordDeletionEvent("vpc", vpc.ID, "vpc.deleted", nil)
	slog.Info("deletion completed", "jobId", jobID, "resourceType", "vpc", "resourceId", vpcID)
	return nil
}

// asyncDeleteLoadBalancer deletes a load balancer.
func (s *Server) asyncDeleteLoadBalancer(jobID, lbID string) error {
	steps := []string{"validate", "disconnect-targets", "delete"}

	// Step 1: Validate
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[0], []string{}, steps[1:], 20)
	lb, err := s.ctrl.Store.LB.Get(lbID, "")
	if err != nil {
		return s.addDeletionError(jobID, steps[0], "load-balancer", lbID, "load balancer not found", false, "Check load balancer ID")
	}

	// Step 2: Disconnect targets (remove from target groups)
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[1], []string{steps[0]}, steps[2:], 50)
	// TODO: Disconnect targets if needed

	// Step 3: Delete LB
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[2], []string{steps[0], steps[1]}, []string{}, 80)

	// Release routable IP if allocated
	if err := s.lbVIPPlacer().ReleaseVIP(lb); err != nil {
		slog.Warn("failed to release vip during lb deletion", "lb", lbID, "error", err)
		// Continue anyway; main deletion will proceed
	}

	if err := s.ctrl.Store.LB.Delete(lbID, ""); err != nil {
		return s.addDeletionError(jobID, steps[2], "load-balancer", lbID, fmt.Sprintf("deletion failed: %v", err), false, "Check load balancer state")
	}

	s.recordDeletionEvent("load-balancer", lb.ID, "load-balancer.deleted", nil)
	return nil
}

// asyncDeleteDatabase deletes a managed database.
func (s *Server) asyncDeleteDatabase(jobID, dbID string) error {
	steps := []string{"validate", "stop-instance", "delete"}

	// Step 1: Validate
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[0], []string{}, steps[1:], 20)
	db, err := s.ctrl.Store.Databases.Get(dbID, "")
	if err != nil {
		return s.addDeletionError(jobID, steps[0], "database", dbID, "database not found", false, "Check database ID")
	}

	// Step 2: Stop backing instance
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[1], []string{steps[0]}, steps[2:], 50)
	if db.InstanceID != "" {
		if _, _, err := s.ctrl.Instances.Stop(db.InstanceID, 5*time.Second, true); err != nil {
			slog.Warn("failed to stop database instance", "instance", db.InstanceID, "error", err)
			// Continue anyway; we'll try to remove it
		}
		if err := s.ctrl.Instances.Remove(db.InstanceID); err != nil {
			return s.addDeletionError(jobID, steps[1], "database", dbID, fmt.Sprintf("cannot remove instance: %v", err), true, "Check instance state")
		}
	}

	// Step 3: Delete DB record (and secret)
	s.ctrl.Store.DeletionJobs.UpdateProgress(jobID, steps[2], []string{steps[0], steps[1]}, []string{}, 80)
	if err := s.ctrl.Store.Databases.Delete(dbID, ""); err != nil {
		return s.addDeletionError(jobID, steps[2], "database", dbID, fmt.Sprintf("deletion failed: %v", err), false, "Check database state")
	}

	s.recordDeletionEvent("database", db.ID, "database.deleted", nil)
	return nil
}

// addDeletionError adds an error to the deletion job and returns it as a function error.
func (s *Server) addDeletionError(jobID, step, resource, resourceID string, reason string, recoverable bool, recovery string) error {
	err := types.DeletionJobError{
		Step:        step,
		Resource:    resource,
		ResourceID:  resourceID,
		Reason:      reason,
		Recoverable: recoverable,
		Recovery:    recovery,
	}
	_ = s.ctrl.Store.DeletionJobs.AddError(jobID, err)
	return fmt.Errorf("%s", err.Reason)
}

// recordDeletionEvent records an audit event for a deleted resource.
// This is a placeholder that should integrate with the actual event recording system.
func (s *Server) recordDeletionEvent(resourceType, resourceID, eventType string, data map[string]any) {
	slog.Info("deletion event", "resourceType", resourceType, "resourceId", resourceID, "eventType", eventType)
	// TODO: Wire up to actual audit event recording
}

// generateJobID creates a unique job ID.
func generateJobID() string {
	return strings.TrimLeft(fmt.Sprintf("%x", time.Now().UnixNano()), "0")
}
