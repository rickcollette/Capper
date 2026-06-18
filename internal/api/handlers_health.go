package api

import (
	"net/http"
	"strconv"

	"capper/internal/store"
	"capper/internal/version"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeData(w, map[string]string{"status": "ok"}, nil)
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	info := version.Get()
	schemaVersion := ""
	if s.ctrl.Store != nil {
		schemaVersion, _ = s.ctrl.Store.SchemaVersion()
	}
	writeData(w, map[string]any{
		"version":         info.Version,
		"commit":          info.Commit,
		"buildDate":       info.BuildDate,
		"goVersion":       info.GoVersion,
		"apiVersion":      info.APIVersion,
		"platform":        info.Platform,
		"schemaVersion":   schemaVersion,
		"minAgentVersion": version.MinAgentVersion,
	}, nil)
}

// handleDBStats exposes the database connection-pool statistics
// (sql.DB.Stats()) so operators can build SLOs and alert on pool saturation —
// a rising WaitCount/WaitDuration means the client pool (or, on the CapDB
// backend, the server --pool-max) is too small. GET /api/v1/db/stats.
func (s *Server) handleDBStats(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "system:metrics:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if s.ctrl.Store == nil || s.ctrl.Store.DB == nil {
		writeData(w, map[string]any{"available": false}, nil)
		return
	}
	st := s.ctrl.Store.DB.Stats()
	writeData(w, map[string]any{
		"available":          true,
		"maxOpenConnections": st.MaxOpenConnections,
		"openConnections":    st.OpenConnections,
		"inUse":              st.InUse,
		"idle":               st.Idle,
		"waitCount":          st.WaitCount,
		"waitDurationMillis": st.WaitDuration.Milliseconds(),
		"maxIdleClosed":      st.MaxIdleClosed,
		"maxIdleTimeClosed":  st.MaxIdleTimeClosed,
		"maxLifetimeClosed":  st.MaxLifetimeClosed,
	}, nil)
}

func (s *Server) handleDaemonStatus(w http.ResponseWriter, _ *http.Request) {
	status := "offline"
	detail := map[string]any{
		"status": status,
		"online": false,
	}
	if s.daemon != nil {
		status = "online"
		detail["status"] = status
		detail["online"] = true
		detail["supervisorStats"] = s.daemon.SupervisorStats()
	}
	writeData(w, detail, nil)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "event:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	opts := store.ListEventsOptions{
		Since:        r.URL.Query().Get("since"),
		ResourceType: r.URL.Query().Get("resourceType"),
		ResourceID:   r.URL.Query().Get("resourceId"),
		Action:       r.URL.Query().Get("action"),
		Limit:        queryInt(r, "limit", 100),
	}
	events, err := s.ctrl.Store.Events.List(opts)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, events, nil)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
