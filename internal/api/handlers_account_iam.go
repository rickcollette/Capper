package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// ---- Account IAM Users -------------------------------------------------------

func (s *Server) handleListAccountIAMUsers(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	users, err := s.ctrl.Store.IAM.IAMStore().ListUsersByAccount(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, users, nil)
}

func (s *Server) handleCreateAccountIAMUser(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	user, _, err := s.ctrl.Store.IAM.IAMStore().CreateUserWithAccount(accountID, req.Name, req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: user})
}

func (s *Server) handleGetAccountIAMUser(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	userID := r.PathValue("userId")
	user, err := s.ctrl.Store.IAM.IAMStore().GetUserByAccount(accountID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeData(w, user, nil)
}

func (s *Server) handlePatchAccountIAMUser(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	userID := r.PathValue("userId")
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().UpdateUserByAccount(accountID, userID, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	user, _ := s.ctrl.Store.IAM.IAMStore().GetUserByAccount(accountID, userID)
	writeData(w, user, nil)
}

func (s *Server) handleDeleteAccountIAMUser(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	userID := r.PathValue("userId")
	if err := s.ctrl.Store.IAM.IAMStore().DeleteUserByAccount(accountID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Account IAM Groups -------------------------------------------------------

func (s *Server) handleListAccountIAMGroups(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	groups, err := s.ctrl.Store.IAM.IAMStore().ListGroupsByAccount(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, groups, nil)
}

func (s *Server) handleCreateAccountIAMGroup(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	group, err := s.ctrl.Store.IAM.IAMStore().CreateGroupByAccount(accountID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: group})
}

func (s *Server) handleGetAccountIAMGroup(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	groupID := r.PathValue("groupId")
	group, err := s.ctrl.Store.IAM.IAMStore().GetGroupByAccount(accountID, groupID)
	if err != nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	writeData(w, group, nil)
}

func (s *Server) handlePatchAccountIAMGroup(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	groupID := r.PathValue("groupId")
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().UpdateGroupByAccount(accountID, groupID, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	group, _ := s.ctrl.Store.IAM.IAMStore().GetGroupByAccount(accountID, groupID)
	writeData(w, group, nil)
}

func (s *Server) handleDeleteAccountIAMGroup(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	groupID := r.PathValue("groupId")
	if err := s.ctrl.Store.IAM.IAMStore().DeleteGroupByAccount(accountID, groupID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddAccountGroupMember(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	groupID := r.PathValue("groupId")
	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().AddGroupMemberByAccount(accountID, groupID, req.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveAccountGroupMember(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	groupID := r.PathValue("groupId")
	userID := r.PathValue("userID")
	if err := s.ctrl.Store.IAM.IAMStore().RemoveGroupMemberByAccount(accountID, groupID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Account IAM Roles -------------------------------------------------------

func (s *Server) handleListAccountIAMRoles(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	roles, err := s.ctrl.Store.IAM.IAMStore().ListRolesByAccount(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, roles, nil)
}

func (s *Server) handleCreateAccountIAMRole(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		TrustPolicy string `json:"trustPolicy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	role, err := s.ctrl.Store.IAM.IAMStore().CreateRoleByAccount(accountID, req.Name, req.Description, req.TrustPolicy)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: role})
}

func (s *Server) handleGetAccountIAMRole(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	roleID := r.PathValue("roleId")
	role, err := s.ctrl.Store.IAM.IAMStore().GetRoleByAccount(accountID, roleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	writeData(w, role, nil)
}

func (s *Server) handlePatchAccountIAMRole(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	roleID := r.PathValue("roleId")
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().UpdateRoleByAccount(accountID, roleID, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	role, _ := s.ctrl.Store.IAM.IAMStore().GetRoleByAccount(accountID, roleID)
	writeData(w, role, nil)
}

func (s *Server) handleDeleteAccountIAMRole(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	roleID := r.PathValue("roleId")
	if err := s.ctrl.Store.IAM.IAMStore().DeleteRoleByAccount(accountID, roleID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Account IAM Service Accounts -------------------------------------------

func (s *Server) handleListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	sas, err := s.ctrl.Store.IAM.IAMStore().ListServiceAccountsByAccount(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, sas, nil)
}

func (s *Server) handleCreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	sa, err := s.ctrl.Store.IAM.IAMStore().CreateServiceAccountByAccount(accountID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: sa})
}

func (s *Server) handleDeleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	saID := r.PathValue("id")
	if err := s.ctrl.Store.IAM.IAMStore().DeleteServiceAccountByAccount(accountID, saID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleIssueServiceAccountToken(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("id")
	bearer, token, err := s.ctrl.Store.IAM.Issue(saID+"-token", "service-account", saID, 365*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: map[string]any{
		"token":     bearer,
		"expiresAt": token.ExpiresAt,
	}})
}

// ---- Account IAM Policies ---------------------------------------------------

func (s *Server) handleListAccountIAMPolicies(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	policies, err := s.ctrl.Store.IAM.IAMStore().ListPoliciesByAccount(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, policies, nil)
}

func (s *Server) handleCreateAccountIAMPolicy(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Document    any    `json:"document"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	docJSON, _ := json.Marshal(req.Document)
	policy, err := s.ctrl.Store.IAM.IAMStore().CreatePolicyByAccount(accountID, req.Name, req.Description, string(docJSON))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: policy})
}

func (s *Server) handleGetAccountIAMPolicy(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	policyID := r.PathValue("id")
	policy, err := s.ctrl.Store.IAM.IAMStore().GetPolicyByAccount(accountID, policyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}
	writeData(w, policy, nil)
}

func (s *Server) handleUpdateAccountIAMPolicy(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	policyID := r.PathValue("id")
	policy, err := s.ctrl.Store.IAM.IAMStore().GetPolicyByAccount(accountID, policyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}
	if policy.Managed {
		writeError(w, http.StatusForbidden, "managed policies are read-only")
		return
	}
	var req struct {
		Document any `json:"document"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	docJSON, _ := json.Marshal(req.Document)
	if err := s.ctrl.Store.IAM.IAMStore().UpdatePolicyDocumentByAccount(accountID, policyID, string(docJSON)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, _ := s.ctrl.Store.IAM.IAMStore().GetPolicyByAccount(accountID, policyID)
	writeData(w, updated, nil)
}

func (s *Server) handleDeleteAccountIAMPolicy(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	policyID := r.PathValue("id")
	policy, err := s.ctrl.Store.IAM.IAMStore().GetPolicyByAccount(accountID, policyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}
	if policy.Managed {
		writeError(w, http.StatusForbidden, "managed policies cannot be deleted")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().DeletePolicyByAccount(accountID, policyID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAttachAccountPolicy(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	policyID := r.PathValue("id")
	var req struct {
		PrincipalType string `json:"principalType"`
		PrincipalID   string `json:"principalId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().AttachPolicyByAccount(accountID, policyID, req.PrincipalType, req.PrincipalID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDetachAccountPolicy(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	policyID := r.PathValue("id")
	var req struct {
		PrincipalType string `json:"principalType"`
		PrincipalID   string `json:"principalId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().DetachPolicyByAccount(accountID, policyID, req.PrincipalType, req.PrincipalID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- IAM Simulate (account-scoped) ------------------------------------------

func (s *Server) handleAccountIAMSimulate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrincipalType string   `json:"principalType"`
		PrincipalID   string   `json:"principalId"`
		Actions       []string `json:"actions"`
		ResourceCRN   string   `json:"resourceCrn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	type SimResult struct {
		Action   string `json:"action"`
		Allowed  bool   `json:"allowed"`
		Decision string `json:"decision"`
	}
	results := make([]SimResult, 0, len(req.Actions))
	for _, action := range req.Actions {
		decision, _, _ := s.ctrl.Store.IAM.IAMStore().Evaluate(req.PrincipalType, req.PrincipalID, action, req.ResourceCRN)
		results = append(results, SimResult{
			Action:   action,
			Allowed:  decision == "allow",
			Decision: decision,
		})
	}
	writeData(w, results, nil)
}

// ---- Role assume (account-scoped) -------------------------------------------

func (s *Server) handleAccountAssumeRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("roleId")
	var req struct {
		SessionName     string `json:"sessionName"`
		DurationSeconds int    `json:"durationSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.DurationSeconds <= 0 || req.DurationSeconds > 3600 {
		req.DurationSeconds = 3600
	}
	if req.SessionName == "" {
		req.SessionName = "session"
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
			"roleId":      roleID,
			"sessionName": req.SessionName,
		},
	}})
}
