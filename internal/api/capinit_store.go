package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func sanitizeID(id string) (string, error) {
	clean := filepath.Base(id)
	if clean == "." || clean == ".." || strings.ContainsAny(clean, "/\\") {
		return "", fmt.Errorf("invalid id: %q", id)
	}
	return clean, nil
}

type CapInitTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func capinitDir(s *Server) string {
	return filepath.Join(s.ctrl.Store.Paths.Root, "capinit", "templates")
}

func capinitTemplateCount(s *Server) int {
	templates, _ := listCapInitTemplates(s)
	return len(templates)
}

func (s *Server) handleListCapInitTemplates(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capinit:template:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	templates, err := listCapInitTemplates(s)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, templates, nil)
}

func (s *Server) handleGetCapInitTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := sanitizeID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.authorize(r, "capinit:template:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	t, ok := loadCapInitTemplate(s, id)
	if !ok {
		writeNotFound(w, "template not found")
		return
	}
	writeData(w, t, nil)
}

func (s *Server) handleCreateCapInitTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capinit:template:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req CapInitTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.ID == "" {
		req.ID = strings.ReplaceAll(req.Name, " ", "-")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	req.CreatedAt = now
	req.UpdatedAt = now
	if err := saveCapInitTemplate(s, req); err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: req})
}

func (s *Server) handleUpdateCapInitTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := sanitizeID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.authorize(r, "capinit:template:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	existing, ok := loadCapInitTemplate(s, id)
	if !ok {
		writeNotFound(w, "template not found")
		return
	}
	var req CapInitTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	req.ID = existing.ID
	req.CreatedAt = existing.CreatedAt
	req.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveCapInitTemplate(s, req); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, req, nil)
}

func (s *Server) handleDeleteCapInitTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := sanitizeID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.authorize(r, "capinit:template:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	path := filepath.Join(capinitDir(s), id+".json")
	if err := os.Remove(path); err != nil {
		writeNotFound(w, "template not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listCapInitTemplates(s *Server) ([]CapInitTemplate, error) {
	dir := capinitDir(s)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []CapInitTemplate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		t, ok := loadCapInitTemplateFile(filepath.Join(dir, e.Name()))
		if ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func loadCapInitTemplate(s *Server, id string) (CapInitTemplate, bool) {
	path := filepath.Join(capinitDir(s), id+".json")
	return loadCapInitTemplateFile(path)
}

func loadCapInitTemplateFile(path string) (CapInitTemplate, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CapInitTemplate{}, false
	}
	var t CapInitTemplate
	if err := json.Unmarshal(data, &t); err != nil {
		return CapInitTemplate{}, false
	}
	return t, true
}

func saveCapInitTemplate(s *Server, t CapInitTemplate) error {
	id, err := sanitizeID(t.ID)
	if err != nil {
		return err
	}
	dir := capinitDir(s)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, id+".json"), append(data, '\n'), 0o644)
}

func instanceMetaDir(s *Server) string {
	return filepath.Join(s.ctrl.Store.Paths.Root, "capinit", "instance-metadata")
}

func loadInstanceMetadata(s *Server, instanceID string) (map[string]any, bool) {
	safeID, err := sanitizeID(instanceID)
	if err != nil {
		return nil, false
	}
	path := filepath.Join(instanceMetaDir(s), safeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, false
	}
	return meta, true
}

func saveInstanceMetadata(s *Server, instanceID string, meta map[string]any) error {
	safeID, err := sanitizeID(instanceID)
	if err != nil {
		return err
	}
	dir := instanceMetaDir(s)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, safeID+".json"), append(data, '\n'), 0o644)
}
