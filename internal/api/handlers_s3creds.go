package api

import (
	"encoding/json"
	"errors"
	"net/http"

	caps3 "capper/internal/s3server"
)

// GET /api/v1/s3/credentials?account=<accountID>
func (s *Server) handleListS3Credentials(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "s3:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	accountID := r.URL.Query().Get("account")
	if accountID == "" {
		writeBadRequest(w, errors.New("account query parameter is required"))
		return
	}
	creds, err := caps3.ListS3Credentials(s.ctrl.Store.DB, accountID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	// Never expose secret keys in list responses.
	for i := range creds {
		creds[i].SecretKey = ""
	}
	writeData(w, creds, nil)
}

// POST /api/v1/s3/credentials
func (s *Server) handleCreateS3Credential(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "s3:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		AccountID string `json:"accountID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.AccountID == "" {
		writeBadRequest(w, errors.New("accountID is required"))
		return
	}
	cred, err := caps3.GenerateS3Credential(s.ctrl.Store.DB, s.ctrl.Store.SecretKey, req.AccountID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	// SecretKey is returned only once at creation time.
	s.recordEvent(r, "s3_credential", cred.ID, "s3.credential.created", map[string]any{"accountID": req.AccountID})
	writeJSON(w, http.StatusCreated, Envelope{Data: cred})
}

// DELETE /api/v1/s3/credentials/{id}
func (s *Server) handleDeleteS3Credential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.authorize(r, "s3:delete", "s3/credential/"+id); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := caps3.DeleteS3Credential(s.ctrl.Store.DB, id); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "s3_credential", id, "s3.credential.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}
