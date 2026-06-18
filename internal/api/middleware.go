package api

import (
	"crypto/hmac"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"capper/internal/authz"
	"capper/internal/iam"
)

// Trusted reverse-proxy identity headers. The secret is a shared value injected
// by nginx (never reaches the browser); the email is set by oauth2-proxy.
const (
	proxySecretHeader = "X-Capper-Proxy-Secret"
	proxyEmailHeader  = "X-Auth-Request-Email"
)

// emailDomainAllowed reports whether email's domain is permitted for
// proxy-authenticated identities. An empty allowlist permits any domain.
func (s *Server) emailDomainAllowed(email string) bool {
	if len(s.allowedEmailDomains) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	dom := strings.ToLower(email[at+1:])
	for _, d := range s.allowedEmailDomains {
		if strings.ToLower(strings.TrimSpace(d)) == dom {
			return true
		}
	}
	return false
}

func (s *Server) chain(next http.Handler) http.Handler {
	h := next
	h = s.csrfMiddleware(h)
	h = s.authMiddleware(h)
	h = s.corsMiddleware(h)
	h = s.loggingMiddleware(h)
	return h
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("api %s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

// originAllowed reports whether a cross-origin request from origin may receive
// credentialed CORS access. Loopback origins are always permitted (local dev /
// the Vite console talking to a localhost API); any other origin must be in the
// operator-configured allowlist. We never reflect an arbitrary origin while also
// setting Access-Control-Allow-Credentials.
func (s *Server) originAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, a := range s.allowedOrigins {
		if a == origin {
			return true
		}
	}
	return isLoopbackOrigin(origin)
}

// isLoopbackOrigin reports whether origin's host is localhost / 127.0.0.1 / ::1
// (any scheme, any port).
func isLoopbackOrigin(origin string) bool {
	host := origin
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	// Strip port (handle bracketed IPv6 too).
	if strings.HasPrefix(host, "[") {
		if i := strings.Index(host, "]"); i >= 0 {
			host = host[1:i]
		}
	} else if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Vary on Origin so caches don't serve one origin's CORS headers to another.
		w.Header().Add("Vary", "Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, X-Capper-Account-ID, X-Capper-Org-ID, X-Capper-Project-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Static assets and non-API paths pass through without auth.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/health" || r.URL.Path == "/api/v1/openapi.json" ||
			r.URL.Path == "/api/v1/version" || r.URL.Path == "/api/v1/daemon/status" {
			next.ServeHTTP(w, r)
			return
		}
		// The session endpoint is self-authenticating: POST validates the bearer
		// token in its body before issuing a cookie, GET/DELETE only read/clear
		// the caller's own cookie. It must be reachable without a prior session,
		// otherwise the cookie-login flow can never start.
		if r.URL.Path == "/api/v1/auth/session" {
			next.ServeHTTP(w, r)
			return
		}
		// Local username/password login is the pre-session entry point.
		if r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		// Node join uses join tokens — skip bearer auth.
		if r.URL.Path == "/api/v1/nodes/join" {
			next.ServeHTTP(w, r)
			return
		}
		// Trusted reverse-proxy identity (oauth2-proxy at the edge). When a proxy
		// secret is configured and present, authenticate as the forwarded SSO
		// user: create on first sight (first user ever => admin), enforce the
		// allowed email domains, and gate on approval status.
		if s.proxySecret != "" && r.Header.Get(proxySecretHeader) != "" {
			if !hmac.Equal([]byte(r.Header.Get(proxySecretHeader)), []byte(s.proxySecret)) {
				writeError(w, http.StatusForbidden, "invalid proxy credentials")
				return
			}
			email := strings.ToLower(strings.TrimSpace(r.Header.Get(proxyEmailHeader)))
			if email == "" {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !s.emailDomainAllowed(email) {
				writeError(w, http.StatusForbidden, "email domain not allowed")
				return
			}
			if s.ctrl.Store.IAM == nil {
				writeError(w, http.StatusInternalServerError, "iam unavailable")
				return
			}
			// No self-registration: only an existing, active user may sign in.
			u, err := s.ctrl.Store.IAM.ResolveSSOUser(email)
			if err != nil {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			ac, err := s.buildAuthContext(r, iam.PrincipalUser, u.ID)
			if err != nil {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			if s.accountSuspended(ac.AccountID) {
				writeError(w, http.StatusForbidden, "account suspended")
				return
			}
			ctx := withAuthContext(withPrincipal(r.Context(), iam.PrincipalUser, u.ID), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			if s.ctrl.Store.IAM != nil {
				pt, pid, err := s.ctrl.Store.IAM.Verify(token)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "invalid token")
					return
				}
				ac, err := s.buildAuthContext(r, pt, pid)
				if err != nil {
					writeError(w, http.StatusForbidden, err.Error())
					return
				}
				if s.accountSuspended(ac.AccountID) {
					writeError(w, http.StatusForbidden, "account suspended")
					return
				}
				ctx := withAuthContext(withPrincipal(r.Context(), pt, pid), ac)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" && s.ctrl.Store.IAM != nil {
			pt, pid, err := s.ctrl.Store.IAM.Verify(c.Value)
			if err == nil {
				ac, err := s.buildAuthContext(r, pt, pid)
				if err != nil {
					writeError(w, http.StatusForbidden, err.Error())
					return
				}
				if s.accountSuspended(ac.AccountID) {
					writeError(w, http.StatusForbidden, "account suspended")
					return
				}
				ctx := withAuthContext(withPrincipal(r.Context(), pt, pid), ac)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

// accountSuspended reports whether the given account exists and is suspended.
func (s *Server) accountSuspended(accountID string) bool {
	if accountID == "" || s.ctrl.Store.Projects == nil {
		return false
	}
	acct, err := s.ctrl.Store.Projects.GetAccount(accountID)
	return err == nil && acct.Status == "suspended"
}

// buildAuthContext resolves the principal's effective org/account/project scope.
//
// Tenant-scope request headers (X-Capper-Account-ID / -Org-ID / -Project-ID) are
// honored only when the verified principal is actually authorized for the
// requested scope — a system principal, the relevant org/account root, or an
// active member of the account. A header may never *widen* a principal's scope;
// an unauthorized scope request is rejected (caller returns 403) rather than
// silently downgraded, so the client gets a clear signal instead of operating in
// an unexpected context.
func (s *Server) buildAuthContext(r *http.Request, pt, pid string) (authz.AuthContext, error) {
	ac := authz.AuthContext{
		PrincipalType: pt,
		PrincipalID:   pid,
		OrgID:         "org_local",
		AccountID:     "acct_local",
		SourceIP:      r.RemoteAddr,
		UserAgent:     r.Header.Get("User-Agent"),
	}
	isSystem := pt == "system"
	projects := s.ctrl.Store.Projects

	// Resolve requested org. Only system or an org-root of the current org may
	// switch org context.
	if reqOrg := r.Header.Get("X-Capper-Org-ID"); reqOrg != "" && reqOrg != ac.OrgID {
		if !(isSystem || s.principalIsOrgRoot(pid, ac.OrgID)) {
			return ac, fmt.Errorf("not authorized for org %q", reqOrg)
		}
		ac.OrgID = reqOrg
	}

	// Resolve requested account. System, an org-root of the account's org, an
	// account-root, or an active member may act on the account.
	if reqAcct := r.Header.Get("X-Capper-Account-ID"); reqAcct != "" && reqAcct != ac.AccountID {
		if !(isSystem || s.principalCanActOnAccount(pid, reqAcct, ac.OrgID)) {
			return ac, fmt.Errorf("not authorized for account %q", reqAcct)
		}
		ac.AccountID = reqAcct
	}

	// Compute root flags against the *resolved* org/account, never the header.
	ac.IsOrgRoot = isSystem || s.principalIsOrgRoot(pid, ac.OrgID)
	ac.IsAccountRoot = s.principalIsAccountRoot(pid, ac.AccountID)

	// Resolve requested project — it must belong to the resolved account.
	if reqProj := r.Header.Get("X-Capper-Project-ID"); reqProj != "" {
		if projects != nil {
			proj, err := projects.GetProject(reqProj)
			if err != nil {
				return ac, fmt.Errorf("not authorized for project %q", reqProj)
			}
			if proj.AccountID != "" && proj.AccountID != ac.AccountID {
				return ac, fmt.Errorf("not authorized for project %q", reqProj)
			}
		}
		ac.ProjectID = reqProj
	}
	return ac, nil
}

// principalIsOrgRoot reports whether pid is a root user of orgID.
func (s *Server) principalIsOrgRoot(pid, orgID string) bool {
	if s.ctrl.Store.Projects == nil {
		return false
	}
	users, err := s.ctrl.Store.Projects.ListOrgRootUsers(orgID)
	if err != nil {
		return false
	}
	for _, u := range users {
		if u.UserID == pid {
			return true
		}
	}
	return false
}

// principalIsAccountRoot reports whether pid is a root user of accountID.
func (s *Server) principalIsAccountRoot(pid, accountID string) bool {
	if s.ctrl.Store.Projects == nil {
		return false
	}
	users, err := s.ctrl.Store.Projects.ListAccountRootUsers(accountID)
	if err != nil {
		return false
	}
	for _, u := range users {
		if u.UserID == pid {
			return true
		}
	}
	return false
}

// principalCanActOnAccount reports whether pid may act within accountID: an
// account root, an active member, or an org-root of the account's org (curOrg is
// the principal's already-resolved org scope).
func (s *Server) principalCanActOnAccount(pid, accountID, curOrg string) bool {
	projects := s.ctrl.Store.Projects
	if projects == nil {
		return false
	}
	if s.principalIsAccountRoot(pid, accountID) {
		return true
	}
	if mems, err := projects.ListMemberships(accountID); err == nil {
		for _, m := range mems {
			if m.UserID == pid && (m.Status == "" || m.Status == "active") {
				return true
			}
		}
	}
	// An org root may act on any account belonging to that org.
	if s.principalIsOrgRoot(pid, curOrg) {
		if acct, err := projects.GetAccount(accountID); err == nil && acct.OrgID == curOrg {
			return true
		}
	}
	return false
}

func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/auth/session" {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := r.Cookie(sessionCookieName); err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}
		csrfCookie, err := r.Cookie(csrfCookieName)
		if err != nil || csrfCookie.Value == "" {
			writeError(w, http.StatusForbidden, "missing CSRF cookie")
			return
		}
		// Constant-time compare to avoid leaking the token via timing.
		if !hmac.Equal([]byte(r.Header.Get(csrfHeaderName)), []byte(csrfCookie.Value)) {
			writeError(w, http.StatusForbidden, "invalid CSRF token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
