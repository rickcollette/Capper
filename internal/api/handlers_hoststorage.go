package api

import (
	"encoding/json"
	"net/http"

	"capper/internal/adminconfig"
	"capper/internal/hoststorage"
)

func (s *Server) hostStorage() *hoststorage.Manager {
	return hoststorage.NewManager(s.ctrl.Store.HostStorage)
}

// GET /api/v1/admin/disks — discovered host disks with allocation state.
func (s *Server) handleListDisks(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:disk:list", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	disks, err := s.hostStorage().Disks()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "disk discovery failed: "+err.Error())
		return
	}
	writeData(w, disks, nil)
}

// GET /api/v1/admin/storage-pools — pools with capacity accounting.
func (s *Server) handleListStoragePools(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:list", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	pools, err := s.hostStorage().ListPools()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, pools, nil)
}

// POST /api/v1/admin/storage-pools — register a pool over a mounted path.
func (s *Server) handleCreateStoragePool(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:create", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name       string `json:"name"`
		Backend    string `json:"backend"`
		Mountpoint string `json:"mountpoint"`
		Device     string `json:"device"`
		VGName     string `json:"vgName"`
		TotalBytes int64  `json:"totalBytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	pool, err := s.hostStorage().CreatePool(hoststorage.CreatePoolOptions{
		Name: req.Name, Backend: req.Backend, Mountpoint: req.Mountpoint,
		Device: req.Device, VGName: req.VGName, TotalBytes: req.TotalBytes,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, pool, nil)
}

// DELETE /api/v1/admin/storage-pools/{id} — remove a pool (must be empty).
func (s *Server) handleDeleteStoragePool(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:delete", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.hostStorage().DeletePool(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"deleted": r.PathValue("id")}, nil)
}

// GET /api/v1/admin/storage-pools/{id}/allocations — claims against a pool.
func (s *Server) handleListStorageAllocations(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	pool, err := s.ctrl.Store.HostStorage.GetPool(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "pool not found")
		return
	}
	allocs, err := s.hostStorage().ListAllocations(pool.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, allocs, nil)
}

// POST /api/v1/admin/storage-pools/{id}/allocations — carve capacity from a pool.
func (s *Server) handleCreateStorageAllocation(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:allocate", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	pool, err := s.ctrl.Store.HostStorage.GetPool(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "pool not found")
		return
	}
	var req struct {
		Name      string `json:"name"`
		Owner     string `json:"owner"`
		SizeBytes int64  `json:"sizeBytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	a, err := s.hostStorage().Allocate(hoststorage.AllocateOptions{
		PoolID: pool.ID, Name: req.Name, Owner: req.Owner, SizeBytes: req.SizeBytes,
	})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, a, nil)
}

// DELETE /api/v1/admin/storage-allocations/{id} — release an allocation.
func (s *Server) handleDeleteStorageAllocation(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:allocate", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.hostStorage().Release(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"released": r.PathValue("id")}, nil)
}

// GET /api/v1/admin/storage/settings — storage defaults (e.g. instance pool).
func (s *Server) handleGetStorageSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	pool, _, _ := s.ctrl.Store.AdminConfig.Get(adminconfig.KeyDefaultInstancePool)
	writeData(w, map[string]any{"defaultInstancePool": pool}, nil)
}

// PUT /api/v1/admin/storage/settings — set storage defaults. An empty
// defaultInstancePool clears it (instance disks revert to the store path).
func (s *Server) handleSetStorageSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:storage:pool:create", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		DefaultInstancePool string `json:"defaultInstancePool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.DefaultInstancePool == "" {
		_ = s.ctrl.Store.AdminConfig.Delete(adminconfig.KeyDefaultInstancePool)
	} else {
		// Validate the pool exists before pinning it.
		if _, err := s.ctrl.Store.HostStorage.GetPool(req.DefaultInstancePool); err != nil {
			writeError(w, http.StatusBadRequest, "storage pool not found")
			return
		}
		if err := s.ctrl.Store.AdminConfig.Set(adminconfig.KeyDefaultInstancePool, req.DefaultInstancePool); err != nil {
			writeInternal(w, err)
			return
		}
	}
	pool, _, _ := s.ctrl.Store.AdminConfig.Get(adminconfig.KeyDefaultInstancePool)
	writeData(w, map[string]any{"defaultInstancePool": pool}, nil)
}
