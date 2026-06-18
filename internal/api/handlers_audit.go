package api

import (
	"net/http"
	"strconv"
)

// handleListAccountAuditEvents handles GET /api/v1/accounts/{account}/audit
func (s *Server) handleListAccountAuditEvents(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "account is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		orgID = "org_local"
	}

	decision := r.URL.Query().Get("decision")

	if decision == "deny" {
		events, err := s.ctrl.Store.Audit.ListDenials(orgID, accountID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeData(w, events, nil)
		return
	}

	events, err := s.ctrl.Store.Audit.ListByAccount(orgID, accountID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, events, nil)
}
