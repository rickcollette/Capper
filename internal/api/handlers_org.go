package api

import (
	"encoding/json"
	"net/http"
	"regexp"
)

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ---- orgs -------------------------------------------------------------------

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	orgs, err := s.ctrl.Store.Projects.ListOrgs()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, orgs, nil)
}

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		BillingEmail string `json:"billingEmail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Slug != "" && !slugRe.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "slug must match ^[a-z0-9-]+$")
		return
	}
	o, err := s.ctrl.Store.Projects.CreateOrg(req.Name)
	if err != nil {
		writeInternal(w, err)
		return
	}
	updates := map[string]string{}
	if req.Slug != "" {
		updates["slug"] = req.Slug
	}
	if req.BillingEmail != "" {
		updates["billing_email"] = req.BillingEmail
	}
	if len(updates) > 0 {
		_ = s.ctrl.Store.Projects.UpdateOrg(o.ID, updates)
		o, _ = s.ctrl.Store.Projects.GetOrg(o.ID)
	}
	s.recordEvent(r, "org", o.ID, "create", nil)
	writeData(w, o, nil)
}

func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("org")
	o, err := s.ctrl.Store.Projects.GetOrg(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	writeData(w, o, nil)
}

func (s *Server) handlePatchOrg(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(id); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if slug, ok := req["slug"]; ok && slug != "" && !slugRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "slug must match ^[a-z0-9-]+$")
		return
	}
	if err := s.ctrl.Store.Projects.UpdateOrg(id, req); err != nil {
		writeInternal(w, err)
		return
	}
	o, _ := s.ctrl.Store.Projects.GetOrg(id)
	s.recordEvent(r, "org", id, "update", nil)
	writeData(w, o, nil)
}

func (s *Server) handleDeleteOrg(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(id); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if err := s.ctrl.Store.Projects.DeleteOrg(id); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "org", id, "delete", nil)
	writeData(w, map[string]string{"id": id}, nil)
}

// ---- accounts ---------------------------------------------------------------

func (s *Server) handleListOrgAccounts(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	accounts, err := s.ctrl.Store.Projects.ListAccounts(orgID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, accounts, nil)
}

func (s *Server) handleCreateOrgAccount(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		AccountType string `json:"accountType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	a, err := s.ctrl.Store.Projects.CreateAccount(orgID, req.Name)
	if err != nil {
		writeInternal(w, err)
		return
	}
	updates := map[string]string{}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.AccountType != "" {
		updates["account_type"] = req.AccountType
	}
	if len(updates) > 0 {
		_ = s.ctrl.Store.Projects.UpdateAccount(a.ID, updates)
		a, _ = s.ctrl.Store.Projects.GetAccount(a.ID)
	}
	s.recordEvent(r, "account", a.ID, "create", nil)
	writeData(w, a, nil)
}

func (s *Server) handleGetOrgAccount(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	a, err := s.ctrl.Store.Projects.GetAccount(accountID)
	if err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	writeData(w, a, nil)
}

func (s *Server) handlePatchOrgAccount(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.ctrl.Store.Projects.UpdateAccount(accountID, req); err != nil {
		writeInternal(w, err)
		return
	}
	a, _ := s.ctrl.Store.Projects.GetAccount(accountID)
	s.recordEvent(r, "account", accountID, "update", nil)
	writeData(w, a, nil)
}

func (s *Server) handleDeleteOrgAccount(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err := s.ctrl.Store.Projects.DeleteAccount(accountID); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "account", accountID, "delete", nil)
	writeData(w, map[string]string{"id": accountID}, nil)
}

func (s *Server) handleSuspendAccount(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err := s.ctrl.Store.Projects.UpdateAccount(accountID, map[string]string{"status": "suspended"}); err != nil {
		writeInternal(w, err)
		return
	}
	a, _ := s.ctrl.Store.Projects.GetAccount(accountID)
	s.recordEvent(r, "account", accountID, "suspend", nil)
	writeData(w, a, nil)
}

func (s *Server) handleReactivateAccount(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err := s.ctrl.Store.Projects.UpdateAccount(accountID, map[string]string{"status": "active"}); err != nil {
		writeInternal(w, err)
		return
	}
	a, _ := s.ctrl.Store.Projects.GetAccount(accountID)
	s.recordEvent(r, "account", accountID, "reactivate", nil)
	writeData(w, a, nil)
}

// ---- org root users ---------------------------------------------------------

func (s *Server) handleListOrgRootUsers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	users, err := s.ctrl.Store.Projects.ListOrgRootUsers(orgID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, users, nil)
}

func (s *Server) handleAddOrgRootUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	var req struct {
		UserID string `json:"userId"`
		Email  string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}
	if req.Email == "" {
		req.Email = req.UserID
	}
	u, err := s.ctrl.Store.Projects.AddOrgRootUser(orgID, req.UserID, req.Email)
	if err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "org-root-user", u.ID, "add", nil)
	writeData(w, u, nil)
}

func (s *Server) handleRemoveOrgRootUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	userID := r.PathValue("userID")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if err := s.ctrl.Store.Projects.RemoveOrgRootUser(orgID, userID); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "org-root-user", userID, "remove", nil)
	writeData(w, map[string]string{"userId": userID}, nil)
}

// ---- account root users -----------------------------------------------------

func (s *Server) handleListAccountRootUsers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	users, err := s.ctrl.Store.Projects.ListAccountRootUsers(accountID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, users, nil)
}

func (s *Server) handleAddAccountRootUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	var req struct {
		UserID string `json:"userId"`
		Email  string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}
	if req.Email == "" {
		req.Email = req.UserID
	}
	u, err := s.ctrl.Store.Projects.AddAccountRootUser(orgID, accountID, req.UserID, req.Email)
	if err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "account-root-user", u.ID, "add", nil)
	writeData(w, u, nil)
}

func (s *Server) handleRemoveAccountRootUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	accountID := r.PathValue("account")
	userID := r.PathValue("userID")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetAccount(accountID); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err := s.ctrl.Store.Projects.RemoveAccountRootUser(accountID, userID); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "account-root-user", userID, "remove", nil)
	writeData(w, map[string]string{"userId": userID}, nil)
}

// ---- guardrails -------------------------------------------------------------

func (s *Server) handleListGuardrails(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	gs, err := s.ctrl.Store.Projects.ListGuardrails(orgID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, gs, nil)
}

func (s *Server) handleCreateGuardrail(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		DocumentJSON string `json:"document"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.DocumentJSON == "" {
		req.DocumentJSON = "{}"
	}
	g, err := s.ctrl.Store.Projects.CreateGuardrail(orgID, req.Name, req.Description, req.DocumentJSON)
	if err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "guardrail", g.ID, "create", nil)
	writeData(w, g, nil)
}

func (s *Server) handleGetGuardrail(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	id := r.PathValue("id")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	g, err := s.ctrl.Store.Projects.GetGuardrail(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "guardrail not found")
		return
	}
	writeData(w, g, nil)
}

func (s *Server) handleDeleteGuardrail(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org")
	id := r.PathValue("id")
	if _, err := s.ctrl.Store.Projects.GetOrg(orgID); err != nil {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if _, err := s.ctrl.Store.Projects.GetGuardrail(id); err != nil {
		writeError(w, http.StatusNotFound, "guardrail not found")
		return
	}
	if err := s.ctrl.Store.Projects.DeleteGuardrail(id); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "guardrail", id, "delete", nil)
	writeData(w, map[string]string{"id": id}, nil)
}
