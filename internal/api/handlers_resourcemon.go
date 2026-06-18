package api

import (
	"encoding/json"
	"net/http"
	"time"

	"capper/internal/resourcemon"
)

func (s *Server) rmon() *resourcemon.Store { return s.ctrl.Store.ResourceMon }

// ---- resource inventory ----------------------------------------------------

// GET /api/v1/resources
func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	f := resourcemon.ResourceFilter{
		Project:      r.URL.Query().Get("project"),
		ResourceType: r.URL.Query().Get("type"),
		Status:       r.URL.Query().Get("status"),
		Health:       r.URL.Query().Get("health"),
		RegionID:     r.URL.Query().Get("region"),
		ZoneID:       r.URL.Query().Get("zone"),
		NodeID:       r.URL.Query().Get("node"),
		Query:        r.URL.Query().Get("q"),
		Limit:        queryInt(r, "limit", 500),
	}
	resources, err := s.rmon().ListResources(f)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, resources, nil)
}

// GET /api/v1/resources/{id}
func (s *Server) handleGetResource(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	res, err := s.rmon().GetResource(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "resource not found")
		return
	}
	writeData(w, res, nil)
}

// GET /api/v1/resources/{id}/config
func (s *Server) handleResourceConfig(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	if r.URL.Query().Get("history") == "true" {
		hist, err := s.rmon().ConfigHistory(id, queryInt(r, "limit", 50))
		if err != nil {
			writeInternal(w, err)
			return
		}
		writeData(w, hist, nil)
		return
	}
	cfg, err := s.rmon().LatestConfig(id)
	if err != nil {
		writeNotFound(w, "no config recorded for resource")
		return
	}
	writeData(w, cfg, nil)
}

// GET /api/v1/resources/{id}/events
func (s *Server) handleResourceEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	res, err := s.rmon().GetResource(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "resource not found")
		return
	}
	events, err := s.rmon().ListEventsByResource(res.ResourceType, res.ID, queryInt(r, "limit", 100))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, events, nil)
}

// GET /api/v1/resources/{id}/metrics
func (s *Server) handleResourceMetrics(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	res, err := s.rmon().GetResource(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "resource not found")
		return
	}
	names, err := s.rmon().MetricNames(res.ResourceType, res.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"resource": res, "metricNames": names}, nil)
}

// POST /api/v1/resources/sync
func (s *Server) handleSyncResources(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:sync", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	results, err := s.ctrl.Store.SyncResourceMonitor(s.project)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, results, nil)
}

// ---- metrics ---------------------------------------------------------------

// POST /api/v1/metrics/ingest
func (s *Server) handleIngestMetric(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "metrics:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Samples []resourcemon.MetricSample `json:"samples"`
		resourcemon.MetricSample
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	samples := req.Samples
	if len(samples) == 0 && req.MetricName != "" {
		samples = []resourcemon.MetricSample{req.MetricSample}
	}
	for _, m := range samples {
		if m.ResourceType == "" || m.ResourceID == "" || m.MetricName == "" {
			writeError(w, http.StatusBadRequest, "each sample requires resourceType, resourceId, metricName")
			return
		}
		if err := s.rmon().InsertSample(m); err != nil {
			writeInternal(w, err)
			return
		}
	}
	writeData(w, map[string]any{"ingested": len(samples)}, nil)
}

// GET /api/v1/metrics/query
func (s *Server) handleQueryMetrics(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "metrics:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	q := resourcemon.MetricQuery{
		ResourceType: r.URL.Query().Get("resourceType"),
		ResourceID:   r.URL.Query().Get("resourceId"),
		MetricName:   r.URL.Query().Get("metric"),
		Limit:        queryInt(r, "limit", 1000),
	}
	if q.ResourceType == "" || q.ResourceID == "" || q.MetricName == "" {
		writeError(w, http.StatusBadRequest, "resourceType, resourceId, and metric are required")
		return
	}
	if rng := r.URL.Query().Get("range"); rng != "" {
		if d, err := time.ParseDuration(rng); err == nil {
			q.Since = time.Now().UTC().Add(-d).Format(time.RFC3339)
		}
	}
	samples, err := s.rmon().QuerySamples(q)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, samples, nil)
}

// ---- events ----------------------------------------------------------------

// POST /api/v1/events
func (s *Server) handleCreateResourceEvent(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "event:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var e resourcemon.ResourceEvent
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeBadRequest(w, err)
		return
	}
	if e.ResourceType == "" || e.ResourceID == "" || e.EventType == "" {
		writeError(w, http.StatusBadRequest, "resourceType, resourceId, eventType are required")
		return
	}
	if err := s.rmon().RecordEvent(e); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, e, nil)
}

// GET /api/v1/events
func (s *Server) handleListResourceEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "event:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	events, err := s.rmon().ListEvents(r.URL.Query().Get("project"), r.URL.Query().Get("severity"), queryInt(r, "limit", 100))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, events, nil)
}

// ---- alerts ----------------------------------------------------------------

// GET /api/v1/alerts
func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	alerts, err := s.rmon().ListAlerts(r.URL.Query().Get("status"), r.URL.Query().Get("project"), queryInt(r, "limit", 200))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, alerts, nil)
}

// GET /api/v1/alerts/rules
func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	rules, err := s.rmon().ListAlertRules(r.URL.Query().Get("project"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, rules, nil)
}

// POST /api/v1/alerts/rules
func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var rule resourcemon.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeBadRequest(w, err)
		return
	}
	if rule.Name == "" || rule.MetricName == "" {
		writeError(w, http.StatusBadRequest, "name and metricName are required")
		return
	}
	created, err := s.rmon().CreateAlertRule(rule)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, created, nil)
}

// PATCH /api/v1/alerts/rules/{id}
func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var fields map[string]any
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.rmon().UpdateAlertRule(r.PathValue("id"), fields); err != nil {
		writeInternal(w, err)
		return
	}
	rule, err := s.rmon().GetAlertRule(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "rule not found")
		return
	}
	writeData(w, rule, nil)
}

// DELETE /api/v1/alerts/rules/{id}
func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.rmon().DeleteAlertRule(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"deleted": r.PathValue("id")}, nil)
}

// POST /api/v1/alerts/{id}/ack
func (s *Server) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.rmon().AckAlert(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	alert, _ := s.rmon().GetAlert(r.PathValue("id"))
	writeData(w, alert, nil)
}

// POST /api/v1/alerts/{id}/resolve
func (s *Server) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "alert:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.rmon().ResolveAlert(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	alert, _ := s.rmon().GetAlert(r.PathValue("id"))
	writeData(w, alert, nil)
}

// ---- drift -----------------------------------------------------------------

// GET /api/v1/config/drift
func (s *Server) handleListDrift(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	drifted, err := s.rmon().ListDrifted()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, drifted, nil)
}

// ---- service-specific monitoring (§6.6) ------------------------------------

// monitoringHandler returns a handler that aggregates the latest metric per
// metric name plus recent events for a resource of the given type. The {id}
// path value is the resource's native ID (the same value metrics are tagged
// with). It powers GET /api/v1/{service}/{id}/monitoring endpoints.
func (s *Server) monitoringHandler(resourceType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorize(r, "resource:read", "project:"+s.project); err != nil {
			writeForbidden(w, err)
			return
		}
		id := r.PathValue("id")
		names, err := s.rmon().MetricNames(resourceType, id)
		if err != nil {
			writeInternal(w, err)
			return
		}
		metrics := make(map[string]any, len(names))
		for _, name := range names {
			if sample, ok := s.rmon().LatestSample(resourceType, id, name); ok {
				metrics[name] = map[string]any{
					"value": sample.Value, "unit": sample.Unit, "sampledAt": sample.SampledAt,
				}
			}
		}
		events, err := s.rmon().ListEventsByResource(resourceType, id, 25)
		if err != nil {
			writeInternal(w, err)
			return
		}
		writeData(w, map[string]any{
			"resourceType": resourceType,
			"resourceId":   id,
			"metrics":      metrics,
			"events":       events,
		}, nil)
	}
}

// POST /api/v1/resources/{id}/drift/repair
func (s *Server) handleRepairDrift(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "resource:write", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	_, pid := principalFromContext(r.Context())
	cfg, err := s.rmon().RepairDrift(r.PathValue("id"), pid)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, cfg, nil)
}
