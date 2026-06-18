package api

import (
	"encoding/json"
	"net/http"

	"capper/internal/ipam"
)

func (s *Server) ipamStore() *ipam.Store     { return s.ctrl.Store.IPAM }
func (s *Server) ipamManager() *ipam.Manager { return ipam.NewManager(s.ctrl.Store.IPAM) }

// ---- pools -----------------------------------------------------------------

// POST /api/v1/ip-pools
func (s *Server) handleCreateIPPool(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ippool:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		ipam.RoutableIPPool
		Excluded []string `json:"excluded"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Name == "" || req.CIDR == "" {
		writeError(w, http.StatusBadRequest, "name and cidr are required")
		return
	}
	pool, count, err := s.ipamManager().CreatePool(ipam.CreatePoolOptions{
		Pool: req.RoutableIPPool, Excluded: req.Excluded,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, map[string]any{"pool": pool, "addresses": count}, nil)
}

// GET /api/v1/ip-pools
func (s *Server) handleListIPPools(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ippool:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	pools, err := s.ipamStore().ListPools()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, pools, nil)
}

// GET /api/v1/ip-pools/{id}
func (s *Server) handleGetIPPool(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ippool:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	pool, err := s.ipamStore().GetPool(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "pool not found")
		return
	}
	ips, _ := s.ipamStore().ListIPs(pool.ID, "")
	writeData(w, map[string]any{"pool": pool, "addresses": ips}, nil)
}

// DELETE /api/v1/ip-pools/{id}
func (s *Server) handleDeleteIPPool(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ippool:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ipamStore().DeletePool(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"deleted": r.PathValue("id")}, nil)
}

// ---- addresses -------------------------------------------------------------

// POST /api/v1/ips/reserve
func (s *Server) handleReserveIP(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ip:reserve", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Pool     string `json:"pool"`
		Project  string `json:"project"`
		Name     string `json:"name"`
		Purpose  string `json:"purpose"`
		Address  string `json:"address"`
		Reserved bool   `json:"reserved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Pool == "" {
		writeError(w, http.StatusBadRequest, "pool is required")
		return
	}
	if req.Project == "" {
		req.Project = s.project
	}
	pool, err := s.ipamStore().GetPool(req.Pool)
	if err != nil {
		writeNotFound(w, "pool not found")
		return
	}
	ip, err := s.ipamManager().Reserve(ipam.ReserveOptions{
		PoolID: pool.ID, Project: req.Project, Name: req.Name, Purpose: req.Purpose,
		Address: req.Address, Reserved: req.Reserved,
	})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, ip, nil)
}

// GET /api/v1/ips
func (s *Server) handleListIPs(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ip:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	poolID := r.URL.Query().Get("pool")
	if poolID != "" {
		if pool, err := s.ipamStore().GetPool(poolID); err == nil {
			poolID = pool.ID
		}
	}
	ips, err := s.ipamStore().ListIPs(poolID, r.URL.Query().Get("status"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, ips, nil)
}

// GET /api/v1/ips/{id}
func (s *Server) handleGetIP(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ip:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	ip, err := s.ipamStore().GetIP(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "address not found")
		return
	}
	bindings, _ := s.ipamStore().ListBindings(ip.ID)
	writeData(w, map[string]any{"ip": ip, "bindings": bindings}, nil)
}

// POST /api/v1/ips/{id}/release
func (s *Server) handleReleaseIP(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ip:release", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ipamManager().Release(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"released": r.PathValue("id")}, nil)
}

// POST /api/v1/ips/{id}/attach
func (s *Server) handleAttachIP(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ip:attach", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var b ipam.IPBinding
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeBadRequest(w, err)
		return
	}
	if b.TargetType == "" || b.TargetID == "" || b.BindingMode == "" {
		writeError(w, http.StatusBadRequest, "targetType, targetId, and bindingMode are required")
		return
	}
	binding, err := s.ipamManager().Attach(r.PathValue("id"), b)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, binding, nil)
}

// POST /api/v1/ips/{id}/detach
func (s *Server) handleDetachIP(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "ip:detach", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ipamManager().Detach(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"detached": r.PathValue("id")}, nil)
}
