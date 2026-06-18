package api

import (
	"encoding/json"
	"net/http"
	"time"

	"capper/internal/compute"
)

// GPU handlers operate on the SQLite-backed compute GPU store
// (compute_gpu_devices), the same backend used by the `capper compute gpu` CLI
// and the SDK, so registrations are consistent across every interface.

func (s *Server) handleListGPUs(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "gpu:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	devices, err := s.ctrl.Store.Compute.ListGPUDevices()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, devices, nil)
}

func (s *Server) handleAddGPU(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "gpu:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req compute.GPUDevice
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.ID == "" {
		id, _ := randomHex(6)
		req.ID = "gpu-" + id
	}
	if req.Status == "" {
		req.Status = "available"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if req.CreatedAt == "" {
		req.CreatedAt = now
	}
	req.UpdatedAt = now
	if err := s.ctrl.Store.Compute.UpsertGPUDevice(req); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "gpu", req.ID, "gpu.registered", nil)
	writeJSON(w, http.StatusCreated, Envelope{Data: req})
}

func (s *Server) handleDeleteGPU(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "gpu:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	if _, err := s.ctrl.Store.Compute.GetGPUDevice(id); err != nil {
		writeNotFound(w, "gpu device not found")
		return
	}
	if err := s.ctrl.Store.Compute.DeleteGPUDevice(id); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "gpu", id, "gpu.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReleaseGPU(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "gpu:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	if _, err := s.ctrl.Store.Compute.GetGPUDevice(id); err != nil {
		writeNotFound(w, "gpu device not found")
		return
	}
	if err := s.ctrl.Store.Compute.UpdateGPUStatus(id, "available", "", time.Now().UTC().Format(time.RFC3339)); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "gpu", id, "gpu.released", nil)
	writeData(w, map[string]string{"status": "available", "id": id}, nil)
}

func (s *Server) handleAssignGPU(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "gpu:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	var req struct {
		InstanceID string `json:"instanceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if _, err := s.ctrl.Store.Compute.GetGPUDevice(id); err != nil {
		writeNotFound(w, "gpu device not found")
		return
	}
	if err := s.ctrl.Store.Compute.UpdateGPUStatus(id, "assigned", req.InstanceID, time.Now().UTC().Format(time.RFC3339)); err != nil {
		writeInternal(w, err)
		return
	}
	s.recordEvent(r, "gpu", id, "gpu.assigned", map[string]any{"instanceId": req.InstanceID})
	writeData(w, map[string]string{"status": "assigned", "id": id}, nil)
}
