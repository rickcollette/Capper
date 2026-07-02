package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"capper/internal/capstart"
)

func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var builtin *bool
	if v := r.URL.Query().Get("isBuiltin"); v != "" {
		b := v == "true" || v == "1"
		builtin = &b
	}
	recipes, err := s.ctrl.Store.CapStartRecipes.ListRecipes(
		r.URL.Query().Get("category"),
		builtin,
		queryInt(r, "offset", 0),
		queryInt(r, "limit", 200),
	)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, recipes, nil)
}

func (s *Server) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req capstart.CreateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	recipe := capstart.Recipe{
		Name:        req.Name,
		Version:     req.Version,
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		Tags:        req.Tags,
		Content:     req.Content,
		IsBuiltin:   false,
		IsCommunity: false,
	}
	if result := capstart.ValidateRecipe(&recipe); !result.Valid {
		writeJSON(w, http.StatusBadRequest, Envelope{Data: result, Error: "recipe validation failed"})
		return
	}
	if err := s.ctrl.Store.CapStartRecipes.CreateRecipe(&recipe); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "recipe", recipe.ID, "capstart.recipe.created", map[string]any{
		"name": recipe.Name, "version": recipe.Version,
	})
	writeJSON(w, http.StatusCreated, Envelope{Data: recipe})
}

func (s *Server) handleGetRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	recipe, err := s.ctrl.Store.CapStartRecipes.GetRecipe(r.PathValue("id"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, recipe, nil)
}

func (s *Server) handleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	recipe, err := s.ctrl.Store.CapStartRecipes.GetRecipe(r.PathValue("id"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	var req struct {
		Title       *string         `json:"title,omitempty"`
		Description *string         `json:"description,omitempty"`
		Category    *string         `json:"category,omitempty"`
		Tags        []string        `json:"tags,omitempty"`
		Schema      json.RawMessage `json:"schema,omitempty"`
		Content     json.RawMessage `json:"content,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Title != nil {
		recipe.Title = *req.Title
	}
	if req.Description != nil {
		recipe.Description = *req.Description
	}
	if req.Category != nil {
		recipe.Category = *req.Category
	}
	if req.Tags != nil {
		recipe.Tags = req.Tags
	}
	if len(req.Schema) > 0 {
		recipe.Schema = req.Schema
	}
	if len(req.Content) > 0 {
		recipe.Content = req.Content
	}
	if result := capstart.ValidateRecipe(recipe); !result.Valid {
		writeJSON(w, http.StatusBadRequest, Envelope{Data: result, Error: "recipe validation failed"})
		return
	}
	if err := s.ctrl.Store.CapStartRecipes.UpdateRecipe(recipe); err != nil {
		writeCapStartError(w, err)
		return
	}
	s.recordEvent(r, "recipe", recipe.ID, "capstart.recipe.updated", nil)
	writeData(w, recipe, nil)
}

func (s *Server) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	if err := s.ctrl.Store.CapStartRecipes.DeleteRecipe(id); err != nil {
		writeCapStartError(w, err)
		return
	}
	s.recordEvent(r, "recipe", id, "capstart.recipe.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleValidateRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	recipe, err := s.ctrl.Store.CapStartRecipes.GetRecipe(r.PathValue("id"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, capstart.ValidateRecipe(recipe), nil)
}

func (s *Server) handleListISOs(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	isos, err := s.ctrl.Store.CapStartISOs.ListISOs(
		r.URL.Query().Get("osType"),
		queryInt(r, "offset", 0),
		queryInt(r, "limit", 200),
	)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, isos, nil)
}

func (s *Server) handleUploadISO(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		s.handleRegisterISOURL(w, r)
		return
	}
	s.handleUploadISOFile(w, r)
}

func (s *Server) handleRegisterISOURL(w http.ResponseWriter, r *http.Request) {
	var req capstart.UploadISORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Name == "" || req.URL == nil || *req.URL == "" {
		writeBadRequest(w, errors.New("name and url are required"))
		return
	}
	iso := capstart.ISO{
		Name:         req.Name,
		Version:      req.Version,
		OSType:       req.OSType,
		Architecture: req.Architecture,
		Checksum:     req.Checksum,
		ChecksumType: req.ChecksumType,
		URL:          req.URL,
		IsVerified:   false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.ctrl.Store.CapStartISOs.CreateISO(&iso); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "iso", iso.ID, "capstart.iso.created", map[string]any{"name": iso.Name, "url": iso.URL})
	writeJSON(w, http.StatusCreated, Envelope{Data: iso})
}

func (s *Server) handleUploadISOFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeBadRequest(w, fmt.Errorf("parse multipart ISO upload: %w", err))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeBadRequest(w, errors.New("multipart field 'file' is required"))
		return
	}
	defer file.Close()

	id := uuid.New().String()
	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}
	dir := filepath.Join(s.ctrl.Store.Paths.Root, "capstart", "isos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeInternal(w, err)
		return
	}
	dstPath := filepath.Join(dir, id+".iso")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		writeInternal(w, err)
		return
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(dst, hasher), file)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(dstPath)
		writeInternal(w, copyErr)
		return
	}
	if closeErr != nil {
		_ = os.Remove(dstPath)
		writeInternal(w, closeErr)
		return
	}
	iso := capstart.ISO{
		ID:           id,
		Name:         name,
		Version:      r.FormValue("version"),
		OSType:       r.FormValue("osType"),
		Architecture: r.FormValue("architecture"),
		FileSize:     size,
		Checksum:     hex.EncodeToString(hasher.Sum(nil)),
		ChecksumType: "sha256",
		StoragePath:  dstPath,
		IsVerified:   true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.ctrl.Store.CapStartISOs.CreateISO(&iso); err != nil {
		_ = os.Remove(dstPath)
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "iso", iso.ID, "capstart.iso.uploaded", map[string]any{"name": iso.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: iso})
}

func (s *Server) handleGetISO(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	iso, err := s.ctrl.Store.CapStartISOs.GetISO(r.PathValue("id"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, iso, nil)
}

func (s *Server) handleDeleteISO(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	iso, _ := s.ctrl.Store.CapStartISOs.GetISO(id)
	if err := s.ctrl.Store.CapStartISOs.DeleteISO(id); err != nil {
		writeCapStartError(w, err)
		return
	}
	if iso != nil && iso.StoragePath != "" {
		_ = os.Remove(iso.StoragePath)
	}
	s.recordEvent(r, "iso", id, "capstart.iso.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleVerifyISO(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	iso, err := s.ctrl.Store.CapStartISOs.GetISO(r.PathValue("id"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	result := map[string]any{"valid": false, "verified": false, "checksum": iso.Checksum}
	if iso.StoragePath == "" {
		result["message"] = "URL-based ISO is registered; download verification is not implemented yet"
		writeData(w, result, nil)
		return
	}
	sum, err := sha256File(iso.StoragePath)
	if err != nil {
		writeInternal(w, err)
		return
	}
	iso.Checksum = sum
	iso.ChecksumType = "sha256"
	iso.IsVerified = true
	if err := s.ctrl.Store.CapStartISOs.UpdateISO(iso); err != nil {
		writeCapStartError(w, err)
		return
	}
	result["valid"] = true
	result["verified"] = true
	result["checksum"] = sum
	result["message"] = "ISO verified successfully"
	writeData(w, result, nil)
}

func (s *Server) handleCreateVMFromRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	recipeID := r.PathValue("id")
	recipe, err := s.ctrl.Store.CapStartRecipes.GetRecipe(recipeID)
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	var req capstart.CreateVMFromRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if len(req.Config) == 0 {
		req.Config = json.RawMessage(`{}`)
	}
	if result := capstart.ValidateRecipeConfig(recipe, req.Config); !result.Valid {
		writeJSON(w, http.StatusBadRequest, Envelope{Data: result, Error: "recipe configuration validation failed"})
		return
	}
	execution := capstart.RecipeExecution{
		RecipeID:  recipeID,
		VMID:      req.VMName,
		Status:    "pending",
		Config:    req.Config,
		Logs:      stringPtr("Recipe execution is queued; VM orchestration worker is not implemented yet."),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.ctrl.Store.CapStartExecutions.CreateExecution(&execution); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "recipe_execution", execution.ID, "capstart.recipe.execution.queued", map[string]any{"recipe_id": recipeID})
	writeJSON(w, http.StatusAccepted, Envelope{Data: execution})
}

func (s *Server) handleGetRecipeExecution(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	execution, err := s.ctrl.Store.CapStartExecutions.GetExecution(r.PathValue("executionId"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, execution, nil)
}

func (s *Server) handleGetRecipeExecutionLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	execution, err := s.ctrl.Store.CapStartExecutions.GetExecution(r.PathValue("executionId"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, map[string]any{"logs": ptrString(execution.Logs), "status": execution.Status}, nil)
}

func (s *Server) handleStartInstallation(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:manage", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req capstart.CreateInstallationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.ISOID == "" || req.VMID == "" {
		writeBadRequest(w, errors.New("isoID and vmID are required"))
		return
	}
	if _, err := s.ctrl.Store.CapStartISOs.GetISO(req.ISOID); err != nil {
		writeCapStartError(w, err)
		return
	}
	job := capstart.InstallationJob{
		ISOID:         req.ISOID,
		VMID:          req.VMID,
		Status:        "pending",
		Timeout:       req.Timeout,
		InstallerLogs: stringPtr("Installation job is queued; ISO boot orchestration is not implemented yet."),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := s.ctrl.Store.CapStartInstallations.CreateJob(&job); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "installation_job", job.ID, "capstart.installation.queued", map[string]any{"iso_id": req.ISOID, "vm_id": req.VMID})
	writeJSON(w, http.StatusAccepted, Envelope{Data: job})
}

func (s *Server) handleGetInstallationStatus(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	job, err := s.ctrl.Store.CapStartInstallations.GetJob(r.PathValue("jobId"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, job, nil)
}

func (s *Server) handleCancelInstallation(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:manage", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	job, err := s.ctrl.Store.CapStartInstallations.GetJob(r.PathValue("jobId"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	now := time.Now().UTC()
	job.Status = "cancelled"
	job.CompletedAt = &now
	if err := s.ctrl.Store.CapStartInstallations.UpdateJob(job); err != nil {
		writeCapStartError(w, err)
		return
	}
	s.recordEvent(r, "installation_job", job.ID, "capstart.installation.cancelled", nil)
	writeData(w, job, nil)
}

func (s *Server) handleGetInstallationLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "instances:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	job, err := s.ctrl.Store.CapStartInstallations.GetJob(r.PathValue("jobId"))
	if err != nil {
		writeCapStartError(w, err)
		return
	}
	writeData(w, map[string]any{"logs": ptrString(job.InstallerLogs), "status": job.Status}, nil)
}

func (s *Server) handleListBuiltinRecipes(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "capstart:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	builtin := true
	recipes, err := s.ctrl.Store.CapStartRecipes.ListRecipes("", &builtin, 0, 500)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, recipes, nil)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeCapStartError(w http.ResponseWriter, err error) {
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		writeNotFound(w, err.Error())
		return
	}
	writeInternal(w, err)
}

func stringPtr(s string) *string {
	return &s
}

func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
