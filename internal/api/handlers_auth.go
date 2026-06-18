package api

import (
	"encoding/json"
	"net/http"
	"time"

	"capper/internal/iam"
)

const sessionTTL = 24 * time.Hour

// POST /api/v1/auth/login — local username/password login. Public (no prior
// session); on success sets the capper_session cookie. No self-registration:
// only existing, active users with a password can authenticate.
func (s *Server) handleLocalLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	u, err := s.ctrl.Store.IAM.VerifyPassword(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	csrf, err := s.issueSession(w, iam.PrincipalUser, u.ID, sessionTTL)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{
		"user":      u,
		"roles":     s.ctrl.Store.IAM.RolesForUser(u.ID),
		"csrfToken": csrf,
	}, nil)
}

// GET /api/v1/auth/google/callback — completes Google SSO. The reverse-proxy
// identity middleware has already validated the oauth2-proxy session and mapped
// it to an existing active user (ResolveSSOUser); here we mint the capper
// session cookie and redirect into the app. Unknown users never reach this
// (the middleware returns 403).
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	pt, pid := principalFromContext(r.Context())
	if pt != iam.PrincipalUser || pid == "" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if _, err := s.issueSession(w, pt, pid, sessionTTL); err != nil {
		writeInternal(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
