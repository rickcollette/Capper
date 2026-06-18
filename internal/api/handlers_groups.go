package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"capper/internal/autoscale"
	autoscalemetrics "capper/internal/autoscale/metrics"
	autoscalestore "capper/internal/autoscale/store"
	"capper/internal/autoscale/evaluator"
	"capper/internal/compute"
	"capper/internal/manager"
	"capper/internal/types"
)

// computeMgr returns a compute.Manager backed by the shared store.
func (s *Server) computeMgr() *compute.Manager {
	return compute.NewManager(s.ctrl.Store.Compute)
}

// ============================================================
// Compute Groups
// ============================================================

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "group:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	groups, err := s.computeMgr().ListGroups()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, groups, nil)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "group:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Template string `json:"template"`
		Min      int    `json:"min"`
		Desired  int    `json:"desired"`
		Max      int    `json:"max"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	g, err := s.computeMgr().CreateGroup(req.Name, req.Template, req.Min, req.Desired, req.Max)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "group", g.ID, "group.created", map[string]any{"name": req.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: g})
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "group:inspect", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	g, err := s.computeMgr().GetGroup(name)
	if err != nil {
		writeNotFound(w, "group not found")
		return
	}
	writeData(w, g, nil)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "group:delete", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.computeMgr().DeleteGroup(name); err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "group", name, "group.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScaleGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "group:scale", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Desired int `json:"desired"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.computeMgr().ScaleGroup(name, req.Desired); err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "group", name, "group.scaled", map[string]any{"desired": req.Desired})
	writeData(w, map[string]any{"group": name, "desired": req.Desired}, nil)
}

func (s *Server) handleListGroupInstances(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "group:inspect", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	ids, err := s.computeMgr().ListGroupInstances(name)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, ids, nil)
}

func (s *Server) handleReconcileGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "group:reconcile", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	mgr := s.computeMgr()
	statusFn := func(id string) (compute.InstanceStatus, error) {
		inst, err := s.ctrl.Instances.List()
		if err != nil {
			return compute.InstanceStatus{}, err
		}
		for _, i := range inst {
			if i.ID == id {
				return compute.InstanceStatus{ID: i.ID, Status: string(i.Status)}, nil
			}
		}
		return compute.InstanceStatus{}, fmt.Errorf("instance %q not found", id)
	}
	runFn := func(image string, res compute.ResourceSpec, instName string) (string, error) {
		overrides := types.ResourceOverrides{}
		if res.MemoryBytes > 0 {
			overrides.Limits.MemoryBytes = res.MemoryBytes
			overrides.MemorySet = true
		}
		if res.MaxProcesses > 0 {
			overrides.Limits.MaxProcesses = res.MaxProcesses
			overrides.PidsSet = true
		}
		inst, err := s.ctrl.Instances.Run(image, overrides, manager.RunOptions{Name: instName})
		if err != nil {
			return "", err
		}
		return inst.ID, nil
	}
	result, err := mgr.Reconcile(name, statusFn, runFn)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, result, nil)
}

// ============================================================
// Autoscale Policies
// ============================================================

func (s *Server) handleListAutoscalePolicies(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "autoscale:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	policies, err := s.ctrl.Store.Autoscale.Policies.List(s.project)
	if err != nil {
		writeInternal(w, err)
		return
	}
	if policies == nil {
		policies = []autoscale.AutoscalePolicy{}
	}
	writeData(w, policies, nil)
}

func (s *Server) handleCreateAutoscalePolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "autoscale:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name               string  `json:"name"`
		GroupName          string  `json:"group"`
		PolicyType         string  `json:"policyType"`
		MetricName         string  `json:"metricName"`
		MetricScope        string  `json:"metricScope"`
		QueueName          string  `json:"queueName"`
		TargetValue        float64 `json:"targetValue"`
		ScaleOutThreshold  float64 `json:"scaleOutThreshold"`
		ScaleInThreshold   float64 `json:"scaleInThreshold"`
		MinReplicas        int     `json:"minReplicas"`
		MaxReplicas        int     `json:"maxReplicas"`
		ScaleOutStep       int     `json:"scaleOutStep"`
		ScaleInStep        int     `json:"scaleInStep"`
		ScaleOutCooldown   int     `json:"scaleOutCooldownSeconds"`
		ScaleInCooldown    int     `json:"scaleInCooldownSeconds"`
		EvalWindow         int     `json:"evaluationWindowSeconds"`
		StabWindow         int     `json:"stabilizationWindowSeconds"`
		ScheduleJSON       string  `json:"scheduleJson"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	g, err := s.computeMgr().GetGroup(req.GroupName)
	if err != nil {
		writeNotFound(w, "group not found")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	p := autoscale.AutoscalePolicy{
		ID:                   fmt.Sprintf("asp_%d", time.Now().UnixNano()),
		Project:              s.project,
		Name:                 req.Name,
		GroupID:              g.ID,
		GroupName:            g.Name,
		Enabled:              true,
		PolicyType:           orDefault(req.PolicyType, autoscale.PolicyTypeTarget),
		MetricName:           req.MetricName,
		MetricScope:          orDefault(req.MetricScope, autoscale.ScopeGroup),
		QueueName:            req.QueueName,
		TargetValue:          req.TargetValue,
		ScaleOutThreshold:    req.ScaleOutThreshold,
		ScaleInThreshold:     req.ScaleInThreshold,
		MinReplicas:          orDefaultInt(req.MinReplicas, 1),
		MaxReplicas:          orDefaultInt(req.MaxReplicas, 10),
		ScaleOutStep:         orDefaultInt(req.ScaleOutStep, 1),
		ScaleInStep:          orDefaultInt(req.ScaleInStep, 1),
		ScaleOutCooldownSecs: orDefaultInt(req.ScaleOutCooldown, 60),
		ScaleInCooldownSecs:  orDefaultInt(req.ScaleInCooldown, 300),
		EvalWindowSecs:       orDefaultInt(req.EvalWindow, 300),
		StabWindowSecs:       orDefaultInt(req.StabWindow, 300),
		ScheduleJSON:         orDefault(req.ScheduleJSON, "[]"),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.ctrl.Store.Autoscale.Policies.Insert(p); err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "autoscale_policy", p.ID, "autoscale.policy.created",
		map[string]any{"name": p.Name, "group": g.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: p})
}

func (s *Server) handleGetAutoscalePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("policy")
	if err := s.authorize(r, "autoscale:inspect", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	p, err := s.ctrl.Store.Autoscale.Policies.Get(name, s.project)
	if err != nil {
		writeNotFound(w, "policy not found")
		return
	}
	writeData(w, p, nil)
}

func (s *Server) handleUpdateAutoscalePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("policy")
	if err := s.authorize(r, "autoscale:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	p, err := s.ctrl.Store.Autoscale.Policies.Get(name, s.project)
	if err != nil {
		writeNotFound(w, "policy not found")
		return
	}
	// Partial update: decode into p, keeping existing values for zero fields.
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.ctrl.Store.Autoscale.Policies.Update(p); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, p, nil)
}

func (s *Server) handleDeleteAutoscalePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("policy")
	if err := s.authorize(r, "autoscale:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ctrl.Store.Autoscale.Policies.Delete(name, s.project); err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "autoscale_policy", name, "autoscale.policy.deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// Per-group autoscale routes: /api/v1/groups/{name}/autoscale
// ============================================================

func (s *Server) handleGetGroupAutoscale(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "autoscale:inspect", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	g, err := s.computeMgr().GetGroup(name)
	if err != nil {
		writeNotFound(w, "group not found")
		return
	}
	policies, err := s.ctrl.Store.Autoscale.Policies.ForGroup(g.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	if policies == nil {
		policies = []autoscale.AutoscalePolicy{}
	}
	writeData(w, policies, nil)
}

func (s *Server) handleDisableGroupAutoscale(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "autoscale:update", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	g, err := s.computeMgr().GetGroup(name)
	if err != nil {
		writeNotFound(w, "group not found")
		return
	}
	policies, err := s.ctrl.Store.Autoscale.Policies.ForGroup(g.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	for i := range policies {
		p := policies[i]
		p.Enabled = false
		if err := s.ctrl.Store.Autoscale.Policies.Update(p); err != nil {
			writeInternal(w, err)
			return
		}
	}
	writeData(w, map[string]any{"group": name, "disabled": len(policies)}, nil)
}

func (s *Server) handleEvaluateGroupAutoscale(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "autoscale:evaluate", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	g, err := s.computeMgr().GetGroup(name)
	if err != nil {
		writeNotFound(w, "group not found")
		return
	}
	policies, err := s.ctrl.Store.Autoscale.Policies.ForGroup(g.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	if len(policies) == 0 {
		writeData(w, []autoscale.ScaleRecommendation{}, nil)
		return
	}

	collector := s.autoscaleCollector()
	results := make([]autoscale.ScaleRecommendation, 0, len(policies))
	for _, p := range policies {
		ev := s.resolveEvaluator(p.PolicyType, collector)
		if ev == nil {
			continue
		}
		rec, _ := ev.Evaluate(r.Context(), p, g.DesiredSize)
		results = append(results, rec)
	}
	writeData(w, results, nil)
}

func (s *Server) handleListGroupAutoscaleDecisions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "autoscale:inspect", "group/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	g, err := s.computeMgr().GetGroup(name)
	if err != nil {
		writeNotFound(w, "group not found")
		return
	}
	decisions, err := s.ctrl.Store.Autoscale.Decisions.ListForGroup(g.ID, 50)
	if err != nil {
		writeInternal(w, err)
		return
	}
	if decisions == nil {
		decisions = []autoscale.AutoscaleDecision{}
	}
	writeData(w, decisions, nil)
}

// ============================================================
// Custom metrics push  POST /api/v1/metrics/custom
// ============================================================

func (s *Server) handlePushCustomMetric(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "metrics:push", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Scope      string            `json:"scope"`
		ResourceID string            `json:"resource"`
		MetricName string            `json:"metric"`
		Value      float64           `json:"value"`
		Labels     map[string]string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	sample := autoscale.MetricSample{
		ID:         fmt.Sprintf("ms_%d", time.Now().UnixNano()),
		Project:    s.project,
		Scope:      orDefault(req.Scope, autoscale.ScopeCustom),
		ResourceID: req.ResourceID,
		MetricName: req.MetricName,
		Value:      req.Value,
		Labels:     req.Labels,
	}
	if err := s.ctrl.Store.Autoscale.Samples.Insert(sample); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]string{"status": "recorded"}, nil)
}

// ============================================================
// Helpers wired to the autoscale evaluator/collector
// ============================================================

func (s *Server) autoscaleCollector() *autoscalemetrics.Collector {
	return autoscalemetrics.NewCollector(
		s.ctrl.Instances.List,
		func(groupID string) ([]string, error) {
			return s.computeMgr().ListGroupInstances(groupID)
		},
		s.ctrl.Store.LB.RunningStats,
	)
}

func (s *Server) resolveEvaluator(policyType string, collector *autoscalemetrics.Collector) autoscale.Evaluator {
	query := func(ctx context.Context, groupID, metricName string) (float64, error) {
		return collector.QueryMetric(ctx, groupID, metricName, nil)
	}
	switch policyType {
	case autoscale.PolicyTypeTarget:
		return evaluator.NewTargetEvaluator(query)
	case autoscale.PolicyTypeThreshold:
		return evaluator.NewThresholdEvaluator(query)
	case autoscale.PolicyTypeSchedule:
		return evaluator.NewScheduleEvaluator()
	case autoscale.PolicyTypeQueue:
		return evaluator.NewQueueEvaluator(func(ctx context.Context, queueName string) (int64, error) {
			return s.ctrl.Store.Queue.Depth(queueName, s.project)
		})
	default:
		return nil
	}
}

// ---- small helpers ----------------------------------------------------------

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// groupScalerAdapter adapts compute.Manager to autoscale.GroupScaler.
type groupScalerAdapter struct {
	mgr    *compute.Manager
	storeAS *autoscalestore.Store
}

func (a *groupScalerAdapter) CurrentReplicas(_ context.Context, groupID string) (int, error) {
	g, err := a.mgr.GetGroup(groupID)
	if err != nil {
		return 0, err
	}
	return g.DesiredSize, nil
}

func (a *groupScalerAdapter) SetDesiredReplicas(_ context.Context, groupID string, desired int) error {
	return a.mgr.ScaleGroup(groupID, desired)
}
