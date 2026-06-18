package api

import (
	"net/http"

	"capper/internal/controller"
)

func (s *Server) authorize(r *http.Request, action, resource string) error {
	pt, pid := principalFromContext(r.Context())
	if s.ctrl.Store.IAM == nil {
		return nil
	}
	return s.ctrl.Store.IAM.Authorize(pt, pid, action, resource)
}

func (s *Server) allowed(action, resource string, r *http.Request) bool {
	return s.authorize(r, action, resource) == nil
}

// requireAccountIAM wraps an account-scoped IAM handler with an authorization
// check against the target account. Read methods (GET/HEAD) require iam:read;
// all mutating methods require iam:write. Without this guard any authenticated
// principal could manage IAM in any account.
func (s *Server) requireAccountIAM(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := "iam:write"
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			action = "iam:read"
		}
		if err := s.authorize(r, action, "account/"+r.PathValue("account")); err != nil {
			writeForbidden(w, err)
			return
		}
		h(w, r)
	}
}

// requireAccount wraps an account-scoped handler with a fixed-action
// authorization check against the target account resource.
func (s *Server) requireAccount(action string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorize(r, action, "account/"+r.PathValue("account")); err != nil {
			writeForbidden(w, err)
			return
		}
		h(w, r)
	}
}

func instanceCaps(s *Server, r *http.Request, id string) map[string]bool {
	res := "instance/" + id
	return map[string]bool{
		"canStart":   s.allowed("instance:run", res, r),
		"canStop":    s.allowed("instance:stop", res, r),
		"canRestart": s.allowed("instance:stop", res, r) && s.allowed("instance:run", res, r),
		"canDelete":  s.allowed("instance:delete", res, r),
		"canConnect": s.allowed("instance:connect", res, r),
		"canLogs":    s.allowed("instance:logs", res, r),
	}
}

func imageCaps(s *Server, r *http.Request, name string) map[string]bool {
	res := "image/" + name
	return map[string]bool{
		"canDelete":  s.allowed("image:delete", res, r),
		"canRun":     s.allowed("instance:run", "project:"+s.project, r),
		"canPublish": s.allowed("marketplace:publish", res, r),
	}
}

func projectResource(_ controller.Controller, project string) string {
	return "project:" + project
}
