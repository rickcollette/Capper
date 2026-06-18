package api

import (
	"encoding/json"
	"net/http"
	"time"

	"capper/internal/authz"
)

// handleAssumeRole handles POST /api/v1/iam/assume-role
// Accepts a full CRN-based role ARN or a bare role ID.
func (s *Server) handleAssumeRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoleARN         string `json:"roleArn"`
		SessionName     string `json:"sessionName"`
		DurationSeconds int    `json:"durationSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.RoleARN == "" {
		writeError(w, http.StatusBadRequest, "roleArn is required")
		return
	}
	if req.DurationSeconds <= 0 || req.DurationSeconds > 3600 {
		req.DurationSeconds = 3600
	}
	if req.SessionName == "" {
		req.SessionName = "session"
	}

	// Parse CRN to extract role ID. authz.ParseCRN returns:
	// service, realm, region, accountID, resourceType, resourceID, err
	// The resourceID is the specific role ID after "role/".
	// If not a valid CRN, treat the whole string as a role ID.
	roleID := req.RoleARN
	if _, _, _, _, _, resourceID, err := authz.ParseCRN(req.RoleARN); err == nil {
		roleID = resourceID
		if roleID == "" {
			roleID = req.RoleARN
		}
	}

	// Authorize: the caller must be permitted to assume this specific role.
	if err := s.authorize(r, "iam:assumerole", "role/"+roleID); err != nil {
		writeForbidden(w, err)
		return
	}

	ttl := time.Duration(req.DurationSeconds) * time.Second
	bearer, token, err := s.ctrl.Store.IAM.Issue(req.SessionName, "assumed-role", roleID, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: map[string]any{
		"credentials": map[string]any{
			"accessToken": bearer,
			"expiration":  token.ExpiresAt,
		},
		"assumedRoleUser": map[string]any{
			"roleArn":     req.RoleARN,
			"sessionName": req.SessionName,
		},
	}})
}
