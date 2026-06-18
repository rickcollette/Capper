package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"capper/internal/hostsec/provider"
	"capper/internal/hostsec/ufw"
)

// hostsecLocalNode reports whether the host-security request targets the node
// the control plane runs on. AIO has a single "local" node, so an empty or
// "local" selector — or the local node's own ID — is in-process. A request that
// targets a different node belongs to that node's agent (Enterprise): the agent
// runs the same workers and serves them on its local API. We surface that
// clearly instead of silently acting on the wrong host.
func (s *Server) hostsecLocalNode(r *http.Request) (bool, string) {
	node := r.URL.Query().Get("node")
	if node == "" || node == "local" {
		return true, node
	}
	// Resolve the local node's identity from topology; if the selector matches
	// it, execute locally.
	if local, err := s.ctrl.Store.Topology.Store().GetNode("local"); err == nil {
		if node == local.ID || node == local.Slug {
			return true, node
		}
	}
	return false, node
}

// hostsecRequireLocal writes a structured "managed by node agent" response and
// returns false when the request targets a remote node.
func (s *Server) hostsecRequireLocal(w http.ResponseWriter, r *http.Request) bool {
	local, node := s.hostsecLocalNode(r)
	if local {
		return true
	}
	writeError(w, http.StatusConflict,
		"host security for node "+node+" is managed by that node's agent (Enterprise per-node execution)")
	return false
}

// GET /api/v1/admin/hostsec/nodes — nodes the host-security UI can target, with
// the local (in-process) node flagged. Remote nodes are managed by their agents.
func (s *Server) handleHostsecNodes(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	nodes, err := s.ctrl.Store.Topology.Store().ListNodes("")
	if err != nil {
		writeInternal(w, err)
		return
	}
	type nodeView struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Slug  string `json:"slug"`
		Local bool   `json:"local"`
	}
	out := make([]nodeView, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeView{ID: n.ID, Name: n.Name, Slug: n.Slug, Local: n.Slug == "local"})
	}
	writeData(w, out, nil)
}

// ---- fail2ban (admin only) -------------------------------------------------

// GET /api/v1/admin/fail2ban/status
func (s *Server) handleFail2banStatus(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	st, err := provider.Fail2ban().Status(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeData(w, st, nil)
}

// POST /api/v1/admin/fail2ban/ban   {jail, ip}
func (s *Server) handleFail2banBan(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		Jail string `json:"jail"`
		IP   string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := provider.Fail2ban().Ban(r.Context(), req.Jail, req.IP); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"banned": req.IP, "jail": req.Jail}, nil)
}

// POST /api/v1/admin/fail2ban/unban   {jail, ip}
func (s *Server) handleFail2banUnban(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		Jail string `json:"jail"`
		IP   string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := provider.Fail2ban().Unban(r.Context(), req.Jail, req.IP); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	// Removing a manual ban should also drop it from the persistent blocklist so
	// the reconciler does not immediately re-apply it.
	if blocked, _ := s.ctrl.Store.Fail2ban.ListBlocklist(); blocked != nil {
		for _, e := range blocked {
			if e.Jail == req.Jail && e.IP == req.IP {
				_, _ = s.ctrl.Store.Fail2ban.RemoveBlocklist(e.ID)
			}
		}
	}
	writeData(w, map[string]any{"unbanned": req.IP, "jail": req.Jail}, nil)
}

// POST /api/v1/admin/fail2ban/unban-all   {ip}
// Unbans an IP across every jail (system-wide).
func (s *Server) handleFail2banUnbanAll(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.IP == "" {
		writeError(w, http.StatusBadRequest, "ip is required")
		return
	}
	if err := provider.Fail2ban().UnbanAll(r.Context(), req.IP); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	// Drop any matching persistent-blocklist entries so the reconciler doesn't
	// immediately re-ban it.
	if blocked, _ := s.ctrl.Store.Fail2ban.ListBlocklist(); blocked != nil {
		for _, e := range blocked {
			if e.IP == req.IP {
				_, _ = s.ctrl.Store.Fail2ban.RemoveBlocklist(e.ID)
			}
		}
	}
	writeData(w, map[string]any{"unbanned": req.IP, "scope": "all-jails"}, nil)
}

// POST /api/v1/admin/fail2ban/flush — unban every IP from every jail.
func (s *Server) handleFail2banFlush(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	if err := provider.Fail2ban().FlushAll(r.Context()); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"flushed": true}, nil)
}

// POST /api/v1/admin/fail2ban/reload   {jail?}
func (s *Server) handleFail2banReload(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		Jail string `json:"jail"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := provider.Fail2ban().Reload(r.Context(), req.Jail); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"reloaded": true}, nil)
}

// GET /api/v1/admin/fail2ban/blocklist — persistent (always-on) bans.
func (s *Server) handleFail2banBlocklist(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	entries, err := s.ctrl.Store.Fail2ban.ListBlocklist()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, entries, nil)
}

// POST /api/v1/admin/fail2ban/blocklist   {jail, ip, reason}
func (s *Server) handleFail2banAddBlocklist(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		Jail   string `json:"jail"`
		IP     string `json:"ip"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Jail == "" || req.IP == "" {
		writeError(w, http.StatusBadRequest, "jail and ip are required")
		return
	}
	entry, err := s.ctrl.Store.Fail2ban.AddBlocklist(req.Jail, req.IP, req.Reason)
	if err != nil {
		writeInternal(w, err)
		return
	}
	// Apply immediately (best-effort; the reconciler keeps it enforced).
	_ = provider.Fail2ban().Ban(r.Context(), req.Jail, req.IP)
	writeData(w, entry, nil)
}

// DELETE /api/v1/admin/fail2ban/blocklist/{id}
func (s *Server) handleFail2banRemoveBlocklist(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	entry, err := s.ctrl.Store.Fail2ban.RemoveBlocklist(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "blocklist entry not found")
		return
	}
	_ = provider.Fail2ban().Unban(r.Context(), entry.Jail, entry.IP)
	writeData(w, map[string]any{"removed": entry.ID}, nil)
}

// GET /api/v1/admin/fail2ban/allowlist — admin-managed ignoreip entries.
func (s *Server) handleFail2banGetAllowlist(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	ips, err := provider.Fail2ban().GetAllowlist()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"ips": ips}, nil)
}

// PUT /api/v1/admin/fail2ban/allowlist   {ips: [...]}
func (s *Server) handleFail2banSetAllowlist(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:fail2ban:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		IPs []string `json:"ips"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := provider.Fail2ban().SetAllowlist(r.Context(), req.IPs); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"ips": req.IPs}, nil)
}

// ---- UFW (admin only) ------------------------------------------------------

// GET /api/v1/admin/ufw/status
func (s *Server) handleUFWStatus(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:ufw:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	st, err := provider.UFW().Status(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeData(w, st, nil)
}

// POST /api/v1/admin/ufw/rules   {action, port, proto, from, comment}
func (s *Server) handleUFWAddRule(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:ufw:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		Action  string `json:"action"`
		Port    string `json:"port"`
		Proto   string `json:"proto"`
		From    string `json:"from"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.Comment == "" {
		req.Comment = "capper" // tag Capper-managed rules
	}
	if err := provider.UFW().AddRule(r.Context(), ufw.AddRuleOptions{
		Action: req.Action, Port: req.Port, Proto: req.Proto, From: req.From, Comment: req.Comment,
	}); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"status": "ok"}, nil)
}

// DELETE /api/v1/admin/ufw/rules/{num}
func (s *Server) handleUFWDeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:ufw:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	num, err := strconv.Atoi(r.PathValue("num"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule number must be an integer")
		return
	}
	if err := provider.UFW().DeleteRule(r.Context(), num); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeData(w, map[string]any{"deleted": num}, nil)
}

// GET /api/v1/admin/ufw/defaults — default incoming/outgoing/routed policies.
func (s *Server) handleUFWGetDefaults(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:ufw:read", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	d, err := provider.UFW().GetDefaults(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeData(w, d, nil)
}

// PUT /api/v1/admin/ufw/defaults   {direction, policy}
func (s *Server) handleUFWSetDefault(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "admin:hostsec:ufw:write", "admin:system"); err != nil {
		writeForbidden(w, err)
		return
	}
	if !s.hostsecRequireLocal(w, r) {
		return
	}
	var req struct {
		Direction string `json:"direction"`
		Policy    string `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := provider.UFW().SetDefault(r.Context(), req.Direction, req.Policy); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	d, _ := provider.UFW().GetDefaults(r.Context())
	writeData(w, d, nil)
}

// POST /api/v1/admin/ufw/enable  and  /disable
func (s *Server) handleUFWSetEnabled(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorize(r, "admin:hostsec:ufw:write", "admin:system"); err != nil {
			writeForbidden(w, err)
			return
		}
		if err := provider.UFW().SetEnabled(r.Context(), enabled); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeData(w, map[string]any{"enabled": enabled}, nil)
	}
}
