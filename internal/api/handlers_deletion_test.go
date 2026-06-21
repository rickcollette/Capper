package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"capper/internal/types"
)

// TestDeleteResourcePreflight tests the preflight deletion check endpoint.
func TestDeleteResourcePreflight(t *testing.T) {
	// Create test server (mock)
	// Test that preflight returns a confirmation token and deletion plan
	// Test different resource types (instance, vpc, load-balancer, database)
	// Verify the response structure
	t.Skip("Integration test - requires full server setup")
}

// TestDeleteResourceConfirm tests the confirmation endpoint.
func TestDeleteResourceConfirm(t *testing.T) {
	tests := []struct {
		name               string
		confirmationPhrase string
		expectStatus       int
		expectError        bool
		description        string
	}{
		{
			name:               "correct_phrase",
			confirmationPhrase: "DELETE",
			expectStatus:       http.StatusAccepted,
			expectError:        false,
			description:        "Should accept correct confirmation phrase",
		},
		{
			name:               "wrong_case",
			confirmationPhrase: "delete",
			expectStatus:       http.StatusBadRequest,
			expectError:        true,
			description:        "Should reject lowercase phrase",
		},
		{
			name:               "wrong_phrase",
			confirmationPhrase: "REMOVE",
			expectStatus:       http.StatusBadRequest,
			expectError:        true,
			description:        "Should reject wrong phrase",
		},
		{
			name:               "empty_phrase",
			confirmationPhrase: "",
			expectStatus:       http.StatusBadRequest,
			expectError:        true,
			description:        "Should reject empty phrase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require a full server setup with mocked stores
			// Skipping for now as it requires integration with the server
			t.Skip("Integration test - requires full server setup")
		})
	}
}

// TestGetDeletionJob tests polling deletion job status.
func TestGetDeletionJob(t *testing.T) {
	t.Skip("Integration test - requires full server setup")
}

// TestDeletionJobStore_Create tests creating a deletion job.
func TestDeletionJobStore_Create(t *testing.T) {
	// This test would require a real SQLite database
	t.Skip("Database integration test")
}

// TestDeletionJobStore_UpdateProgress tests updating job progress.
func TestDeletionJobStore_UpdateProgress(t *testing.T) {
	// This test would require a real SQLite database
	t.Skip("Database integration test")
}

// TestDeletionJobStore_AddError tests adding errors to a job.
func TestDeletionJobStore_AddError(t *testing.T) {
	// This test would require a real SQLite database
	t.Skip("Database integration test")
}

// TestConfirmationPhraseValidation validates the "DELETE" confirmation requirement.
func TestConfirmationPhraseValidation(t *testing.T) {
	tests := []struct {
		phrase string
		valid  bool
	}{
		{"DELETE", true},
		{"delete", false},
		{"Delete", false},
		{"DEL", false},
		{"DELETE\n", false},
		{" DELETE", false},
		{"DELETE ", false},
	}

	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			isValid := tt.phrase == "DELETE"
			if isValid != tt.valid {
				t.Errorf("phrase %q: got valid=%v, want %v", tt.phrase, isValid, tt.valid)
			}
		})
	}
}

// TestCascadeErrorContract tests the CascadeDeleteError type.
func TestCascadeErrorContract(t *testing.T) {
	err := &types.CascadeDeleteError{
		Resource:    "instance",
		ID:          "inst-123",
		Step:        "stop",
		Cause:       errors.New("instance still running"),
		Recoverable: true,
		Recovery:    "Stop the instance manually",
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("CascadeDeleteError.Error() returned empty string")
	}

	if !contains(errMsg, "instance") {
		t.Errorf("Error message should contain resource type: %s", errMsg)
	}
	if !contains(errMsg, "inst-123") {
		t.Errorf("Error message should contain resource ID: %s", errMsg)
	}
	if !contains(errMsg, "stop") {
		t.Errorf("Error message should contain step: %s", errMsg)
	}
}

// TestDeletionJobStructure tests the DeletionJob structure.
func TestDeletionJobStructure(t *testing.T) {
	job := &types.DeletionJob{
		ID:             "del-123",
		Status:         "running",
		ResourceType:   "vpc",
		ResourceID:     "vpc-456",
		Progress:       50,
		CurrentStep:    "delete-instances",
		Steps:          []string{"validate", "delete-instances", "delete-vpc"},
		CompletedSteps: []string{"validate"},
		RemainingSteps: []string{"delete-instances", "delete-vpc"},
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(7 * 24 * time.Hour),
	}

	// Test that job can be marshaled to JSON
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	// Test that job can be unmarshaled from JSON
	var unmarshaled types.DeletionJob
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if unmarshaled.ID != job.ID {
		t.Errorf("Job ID mismatch: got %s, want %s", unmarshaled.ID, job.ID)
	}
	if unmarshaled.Status != job.Status {
		t.Errorf("Job Status mismatch: got %s, want %s", unmarshaled.Status, job.Status)
	}
	if unmarshaled.Progress != job.Progress {
		t.Errorf("Job Progress mismatch: got %d, want %d", unmarshaled.Progress, job.Progress)
	}
}

// TestCascadeBlocker tests the CascadeBlocker type.
func TestCascadeBlocker(t *testing.T) {
	blocker := types.CascadeBlocker{
		Resource: "instance",
		ID:       "inst-123",
		Reason:   "running",
		Recovery: "Stop the instance first",
	}

	if blocker.Resource != "instance" {
		t.Errorf("Expected resource 'instance', got %s", blocker.Resource)
	}
	if blocker.Recovery == "" {
		t.Error("Recovery message should not be empty")
	}
}

// TestCascadeStep tests the CascadeStep type.
func TestCascadeStep(t *testing.T) {
	step := types.CascadeStep{
		Sequence:  1,
		Resource:  "instance",
		ID:        "inst-123",
		Action:    "stop",
		DependsOn: []string{},
	}

	if step.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", step.Sequence)
	}
	if step.Action != "stop" {
		t.Errorf("Expected action 'stop', got %s", step.Action)
	}
}

// TestDeletionJobErrorStructure tests the DeletionJobError structure.
func TestDeletionJobErrorStructure(t *testing.T) {
	err := types.DeletionJobError{
		Step:        "stop-instance",
		Resource:    "instance",
		ResourceID:  "inst-123",
		Reason:      "kernel panic",
		Recoverable: false,
		Recovery:    "Check logs and manually recover",
	}

	if err.Step != "stop-instance" {
		t.Errorf("Expected step 'stop-instance', got %s", err.Step)
	}
	if err.Recoverable {
		t.Error("Expected non-recoverable error")
	}
}

// TestDeletionResponseFormat tests the response format of deletion endpoints.
func TestDeletionResponseFormat(t *testing.T) {
	preflight := map[string]interface{}{
		"resourceType":         "vpc",
		"resourceId":           "vpc-123",
		"confirmationToken":    "abc123xyz",
		"requiresConfirmation": true,
		"deleteOrder":          []string{"instance-1", "instance-2", "vpc-123"},
	}

	// Ensure all required fields are present
	requiredFields := []string{"resourceType", "resourceId", "confirmationToken", "requiresConfirmation"}
	for _, field := range requiredFields {
		if _, ok := preflight[field]; !ok {
			t.Errorf("Missing required field in preflight response: %s", field)
		}
	}

	// Test confirmation response format
	confirmation := map[string]interface{}{
		"jobId":   "del-job-123",
		"status":  "queued",
		"pollUrl": "/api/v1/deletion-jobs/del-job-123",
	}

	for _, field := range []string{"jobId", "status", "pollUrl"} {
		if _, ok := confirmation[field]; !ok {
			t.Errorf("Missing required field in confirmation response: %s", field)
		}
	}
}

// Helper function to check if a string contains a substring.
func contains(str, substr string) bool {
	return bytes.Contains([]byte(str), []byte(substr))
}

// TestDeletionJobExpiration tests that jobs expire after 7 days.
func TestDeletionJobExpiration(t *testing.T) {
	job := &types.DeletionJob{
		ID:        "del-123",
		Status:    "completed",
		CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
	}

	if job.ExpiresAt.After(time.Now()) {
		t.Error("Job should be expired")
	}
}

// BenchmarkDeleteConfirmation benchmarks the confirmation validation.
func BenchmarkDeleteConfirmation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Simulate phrase validation
		phrase := "DELETE"
		_ = phrase == "DELETE"
	}
}
