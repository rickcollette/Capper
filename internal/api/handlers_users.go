package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"capper/internal/iam"
)

var errInvalidRole = errors.New("invalid role (must be \"admin\" or \"member\")")

// userView is the API representation of a user plus its granted roles.
type userView struct {
	iam.User
	Roles []string `json:"roles"`
}

func (s *Server) userView(u iam.User) userView {
	return userView{User: u, Roles: s.ctrl.Store.IAM.RolesForUser(u.ID)}
}

// GET /api/v1/users — list all users with status + roles (admin only).
func (s *Server) handleListRBACUsers(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:list", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	users, err := s.ctrl.Store.IAM.IAMStore().ListUsers()
	if err != nil {
		writeInternal(w, err)
		return
	}
	views := make([]userView, 0, len(users))
	for _, u := range users {
		views = append(views, s.userView(u))
	}
	writeData(w, views, nil)
}

// GET /api/v1/users/me — the calling principal's own identity, status, and roles.
// Reachable by any authenticated principal so the console can render the right UI
// (e.g. show the admin Users page only to admins).
func (s *Server) handleCurrentUser(w http.ResponseWriter, r *http.Request) {
	pt, pid := principalFromContext(r.Context())
	if pt != iam.PrincipalUser {
		// Non-user principals (API tokens, nodes) have no user record.
		writeData(w, map[string]any{"principalType": pt, "principalId": pid, "isAdmin": pt == "system"}, nil)
		return
	}
	u, err := s.ctrl.Store.IAM.IAMStore().GetUser(pid)
	if err != nil {
		writeNotFound(w, "user not found")
		return
	}
	roles := s.ctrl.Store.IAM.RolesForUser(u.ID)
	isAdmin := false
	for _, role := range roles {
		if role == iam.RoleAdmin {
			isAdmin = true
		}
	}
	writeData(w, map[string]any{"user": u, "roles": roles, "isAdmin": isAdmin}, nil)
}

// POST /api/v1/users — admin-provisions a user (no self-registration). For
// provider "local" a password may be set inline; for "google" an email maps the
// SSO identity. Optionally assigns a role.
func (s *Server) handleCreateRBACUser(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:create", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Provider string `json:"provider"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Role != "" && req.Role != iam.RoleAdmin && req.Role != iam.RoleMember {
		writeBadRequest(w, errInvalidRole)
		return
	}
	u, err := s.ctrl.Store.IAM.CreateManagedUser(req.Name, req.Email, req.Provider)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Password != "" {
		if err := s.ctrl.Store.IAM.SetPassword(u.ID, req.Password); err != nil {
			writeInternal(w, err)
			return
		}
	}
	if req.Role != "" {
		if err := s.ctrl.Store.IAM.AssignRole(u.ID, req.Role); err != nil {
			writeInternal(w, err)
			return
		}
	}
	u, _ = s.ctrl.Store.IAM.IAMStore().GetUser(u.ID)
	s.recordEvent(r, "user", u.ID, "user.created", nil)
	writeData(w, s.userView(u), nil)
}

// POST /api/v1/users/{id}/password — set or clear a user's password. Admin only.
func (s *Server) handleSetUserPassword(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	u, err := s.ctrl.Store.IAM.IAMStore().GetUser(id)
	if err != nil {
		writeNotFound(w, "user not found")
		return
	}
	if err := s.ctrl.Store.IAM.SetPassword(u.ID, req.Password); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"ok": true}, nil)
}

// POST /api/v1/users/{id}/approve — activate a pending user and assign a role
// (default "member"). Admin only.
func (s *Server) handleApproveUser(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	var req struct {
		Role string `json:"role"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Role == "" {
		req.Role = iam.RoleMember
	}
	if req.Role != iam.RoleAdmin && req.Role != iam.RoleMember {
		writeBadRequest(w, errInvalidRole)
		return
	}
	u, err := s.ctrl.Store.IAM.IAMStore().GetUser(id)
	if err != nil {
		writeNotFound(w, "user not found")
		return
	}
	if err := s.ctrl.Store.IAM.AssignRole(u.ID, req.Role); err != nil {
		writeInternal(w, err)
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().SetUserStatus(u.ID, iam.UserStatusActive); err != nil {
		writeInternal(w, err)
		return
	}
	u, _ = s.ctrl.Store.IAM.IAMStore().GetUser(u.ID)
	writeData(w, s.userView(u), nil)
}

// POST /api/v1/users/{id}/disable — revoke access (status=disabled). Admin only.
func (s *Server) handleDisableUser(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	u, err := s.ctrl.Store.IAM.IAMStore().GetUser(id)
	if err != nil {
		writeNotFound(w, "user not found")
		return
	}
	if err := s.ctrl.Store.IAM.IAMStore().SetUserStatus(u.ID, iam.UserStatusDisabled); err != nil {
		writeInternal(w, err)
		return
	}
	u, _ = s.ctrl.Store.IAM.IAMStore().GetUser(u.ID)
	writeData(w, s.userView(u), nil)
}

// POST /api/v1/users/{id}/roles — grant a role to a user. Admin only.
func (s *Server) handleGrantUserRole(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Role != iam.RoleAdmin && req.Role != iam.RoleMember {
		writeBadRequest(w, errInvalidRole)
		return
	}
	u, err := s.ctrl.Store.IAM.IAMStore().GetUser(id)
	if err != nil {
		writeNotFound(w, "user not found")
		return
	}
	if err := s.ctrl.Store.IAM.AssignRole(u.ID, req.Role); err != nil {
		writeInternal(w, err)
		return
	}
	u, _ = s.ctrl.Store.IAM.IAMStore().GetUser(u.ID)
	writeData(w, s.userView(u), nil)
}

// DELETE /api/v1/users/{id}/roles/{role} — revoke a role from a user. Admin only.
func (s *Server) handleRevokeUserRole(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "iam:user:update", "iam:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	role := r.PathValue("role")
	u, err := s.ctrl.Store.IAM.IAMStore().GetUser(id)
	if err != nil {
		writeNotFound(w, "user not found")
		return
	}
	if err := s.ctrl.Store.IAM.RevokeRole(u.ID, role); err != nil {
		writeInternal(w, err)
		return
	}
	u, _ = s.ctrl.Store.IAM.IAMStore().GetUser(u.ID)
	writeData(w, s.userView(u), nil)
}
