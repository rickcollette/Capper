package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "secret:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	secrets, err := s.ctrl.Store.Secrets.List(s.project)
	if err != nil {
		writeInternal(w, err)
		return
	}
	// Strip ciphertext before returning.
	type safeSecret struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		CreatedAt   string `json:"createdAt"`
		UpdatedAt   string `json:"updatedAt"`
	}
	out := make([]safeSecret, len(secrets))
	for i, sec := range secrets {
		out[i] = safeSecret{
			ID:          sec.ID,
			Name:        sec.Name,
			Description: sec.Description,
			CreatedAt:   sec.CreatedAt,
			UpdatedAt:   sec.UpdatedAt,
		}
	}
	writeData(w, out, nil)
}

func (s *Server) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "secret:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Value       string `json:"value"`
		Description string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Name == "" || req.Value == "" {
		writeError(w, http.StatusBadRequest, "name and value are required")
		return
	}
	sec, err := s.ctrl.Store.Secrets.Create(req.Name, s.project, req.Description, req.Value)
	if err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "secret", sec.ID, "secret.created", map[string]any{"name": sec.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: map[string]any{
		"id":          sec.ID,
		"name":        sec.Name,
		"description": sec.Description,
		"createdAt":   sec.CreatedAt,
		"updatedAt":   sec.UpdatedAt,
	}})
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "secret:get", "secret/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	sec, err := s.ctrl.Store.Secrets.Get(name, s.project)
	if err != nil {
		writeNotFound(w, "secret not found")
		return
	}
	writeData(w, map[string]any{
		"id":          sec.ID,
		"name":        sec.Name,
		"description": sec.Description,
		"createdAt":   sec.CreatedAt,
		"updatedAt":   sec.UpdatedAt,
	}, nil)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "secret:delete", "secret/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ctrl.Store.Secrets.Delete(name, s.project); err != nil {
		writeNotFound(w, "secret not found")
		return
	}
	s.recordEvent(r, "secret", name, "secret.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}
