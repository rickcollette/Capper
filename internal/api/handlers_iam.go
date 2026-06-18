package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"capper/internal/iam"
)

func randShortID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (s *Server) handleListIAMUsers(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	users, err := s.ctrl.Store.IAM.IAMStore().ListUsers()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, users, nil)
}

func (s *Server) handleCreateIAMUser(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:create", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name      string `json:"name"`
		LocalUser string `json:"localUser,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	u := iam.User{
		ID:        randShortID("usr_"),
		Name:      req.Name,
		LocalUser: req.LocalUser,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.ctrl.Store.IAM.IAMStore().InsertUser(u); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: u})
}

func (s *Server) handleDeleteIAMUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "iam:user:delete", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().DeleteUser(name); err != nil {
		writeBadRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListIAMGroups(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:group:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	groups, err := listIAMGroups(s)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, groups, nil)
}

func listIAMGroups(s *Server) ([]iam.Group, error) {
	rows, err := s.ctrl.Store.DB.Query(`SELECT id, name, created_at FROM iam_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []iam.Group
	for rows.Next() {
		var g iam.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		if full, err := s.ctrl.Store.IAM.IAMStore().GetGroup(g.Name); err == nil {
			g.Members = full.Members
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Server) handleCreateIAMGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:group:create", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	g := iam.Group{ID: randShortID("grp_"), Name: req.Name, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := s.ctrl.Store.IAM.IAMStore().InsertGroup(g); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: g})
}

func (s *Server) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if err := s.authorize(r, "iam:group:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		User string `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().AddGroupMember(group, req.User); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, map[string]string{"status": "added"}, nil)
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	user := r.PathValue("user")
	if err := s.authorize(r, "iam:group:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().RemoveGroupMember(group, user); err != nil {
		writeBadRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListIAMRoles(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:role:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	roles, err := s.ctrl.Store.IAM.IAMStore().ListRoles()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, roles, nil)
}

func (s *Server) handleCreateIAMRole(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:role:create", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	role := iam.Role{ID: randShortID("role_"), Name: req.Name, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := s.ctrl.Store.IAM.IAMStore().InsertRole(role); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: role})
}

func (s *Server) handleListIAMPolicies(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:policy:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	policies, err := s.ctrl.Store.IAM.IAMStore().ListPolicies()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, policies, nil)
}

func (s *Server) handleCreateIAMPolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:policy:create", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req iam.Policy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.ID == "" {
		req.ID = "pol_" + req.Name
	}
	req.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.ctrl.Store.IAM.IAMStore().InsertPolicy(req); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: req})
}

func (s *Server) handleIAMSimulate(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:simulate", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Action   string `json:"action"`
		Resource string `json:"resource"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	pt, pid := principalFromContext(r.Context())
	decision, policyID, err := s.ctrl.Store.IAM.IAMStore().Evaluate(pt, pid, req.Action, req.Resource)
	if err != nil {
		writeInternal(w, err)
		return
	}
	allowed := decision == iam.DecisionAllow
	reason := ""
	if !allowed {
		reason = fmt.Sprintf("denied by policy evaluation (policy: %s)", policyID)
	}
	writeData(w, map[string]any{"allowed": allowed, "reason": reason, "decision": decision}, nil)
}

func (s *Server) handleIAMAudit(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:audit:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	records, err := s.ctrl.Store.IAM.IAMStore().ListAudit(
		r.URL.Query().Get("action"),
		r.URL.Query().Get("principal"),
		r.URL.Query().Get("since"),
		queryInt(r, "limit", 100),
	)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, records, nil)
}

func (s *Server) handleIssueToken(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:token:create", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
		TTL  string `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.TTL == "" {
		req.TTL = "24h"
	}
	d, err := time.ParseDuration(req.TTL)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	if d > 90*24*time.Hour {
		writeBadRequest(w, fmt.Errorf("token TTL cannot exceed 90 days"))
		return
	}
	pt, pid := principalFromContext(r.Context())
	bearer, tok, err := s.ctrl.Store.IAM.Issue(req.Name, pt, pid, d)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: map[string]any{
		"token":  tok,
		"bearer": bearer,
	}})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:token:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	pt, pid := principalFromContext(r.Context())
	tokens, err := s.ctrl.Store.IAM.IAMStore().ListTokens(pt, pid)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, tokens, nil)
}
