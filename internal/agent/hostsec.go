package agent

import (
	"encoding/json"
	"net/http"
	"strconv"

	"capper/internal/hostsec/provider"
	"capper/internal/hostsec/ufw"
)

// Host-security endpoints let the agent manage the host OS it runs on. They use
// the same process-wide exclusive workers as the control daemon, so this node's
// fail2ban/UFW are always driven through a single serialized queue. This is the
// per-node execution path for the Enterprise (multi-node) profile; in AIO the
// control daemon serves these directly instead.

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (l *LocalAPI) handleF2BStatus(w http.ResponseWriter, r *http.Request) {
	st, err := provider.Fail2ban().Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (l *LocalAPI) handleF2BBan(w http.ResponseWriter, r *http.Request) {
	var req struct{ Jail, IP string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := provider.Fail2ban().Ban(r.Context(), req.Jail, req.IP); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"banned": req.IP, "jail": req.Jail})
}

func (l *LocalAPI) handleF2BUnban(w http.ResponseWriter, r *http.Request) {
	var req struct{ Jail, IP string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := provider.Fail2ban().Unban(r.Context(), req.Jail, req.IP); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unbanned": req.IP, "jail": req.Jail})
}

func (l *LocalAPI) handleUFWStatus(w http.ResponseWriter, r *http.Request) {
	st, err := provider.UFW().Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (l *LocalAPI) handleUFWAddRule(w http.ResponseWriter, r *http.Request) {
	var o ufw.AddRuleOptions
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if o.Comment == "" {
		o.Comment = "capper"
	}
	if err := provider.UFW().AddRule(r.Context(), o); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (l *LocalAPI) handleUFWDeleteRule(w http.ResponseWriter, r *http.Request) {
	num, err := strconv.Atoi(r.PathValue("num"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rule number must be an integer"})
		return
	}
	if err := provider.UFW().DeleteRule(r.Context(), num); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": num})
}
