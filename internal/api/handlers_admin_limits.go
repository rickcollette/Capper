package api

import (
	"encoding/json"
	"net/http"

	"capper/internal/adminconfig"
	"capper/internal/deploylimit"
)

// hostLimit is the API view of a single host-wide limit: the effective cap, the
// admin-set override (0 = unset), the built-in default, and current usage.
type hostLimit struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Limit    int64  `json:"limit"`            // effective cap in force
	Override int64  `json:"override"`         // admin override (0 = unset/auto)
	Default  int64  `json:"default"`          // built-in default when unset
	Used     int64  `json:"used"`             // current usage
	Unit     string `json:"unit,omitempty"`
}

// GET /api/v1/admin/limits/host — host-wide limits with usage (admin only).
func (s *Server) handleGetHostLimits(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:limits:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	writeData(w, s.hostLimits(), nil)
}

// PUT /api/v1/admin/limits/host — set host-wide limit overrides (admin only).
// A nil/zero/absent value clears the override (reverts to the default).
func (s *Server) handleSetHostLimits(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:limits:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	var req map[string]*int64
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	cfg := s.ctrl.Store.AdminConfig
	for _, key := range adminconfig.HostLimitKeys {
		val, present := req[key]
		if !present {
			continue
		}
		if val == nil || *val <= 0 {
			if err := cfg.Delete(key); err != nil {
				writeInternal(w, err)
				return
			}
			continue
		}
		if err := cfg.SetInt(key, *val); err != nil {
			writeInternal(w, err)
			return
		}
	}
	writeData(w, s.hostLimits(), nil)
}

// hostLimits assembles the current host-limit views.
func (s *Server) hostLimits() []hostLimit {
	cfg := s.ctrl.Store.AdminConfig
	override := int64(0)
	if n, ok, err := cfg.GetInt(adminconfig.KeyHostDeploymentsMax); err == nil && ok {
		override = n
	}
	used := int64(0)
	if insts, err := s.ctrl.Store.ListInstances(); err == nil {
		used = int64(len(insts))
	}
	return []hostLimit{
		{
			Key:      adminconfig.KeyHostDeploymentsMax,
			Label:    "Max capsule deployments",
			Limit:    s.ctrl.Store.HostDeploymentCap(),
			Override: override,
			Default:  deploylimit.MaxDeployments(),
			Used:     used,
			Unit:     "capsules",
		},
	}
}
