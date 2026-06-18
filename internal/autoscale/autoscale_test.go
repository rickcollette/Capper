package autoscale_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/autoscale"
	autoscalestore "capper/internal/autoscale/store"
	"capper/internal/autoscale/evaluator"
)

// ---- store helpers ----------------------------------------------------------

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Need compute_groups table for the ALTER TABLE statements in autoscale schema.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS compute_groups (
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL UNIQUE,
		template_id  TEXT NOT NULL DEFAULT '',
		min_size     INTEGER NOT NULL DEFAULT 0,
		desired_size INTEGER NOT NULL DEFAULT 1,
		max_size     INTEGER NOT NULL DEFAULT 1,
		status       TEXT NOT NULL DEFAULT 'active',
		created_at   TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatalf("create compute_groups: %v", err)
	}
	if err := autoscalestore.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestStore(t *testing.T) *autoscalestore.Store {
	t.Helper()
	return autoscalestore.New(openTestDB(t))
}

func newPolicy(t *testing.T, st *autoscalestore.Store, overrides ...func(*autoscale.AutoscalePolicy)) autoscale.AutoscalePolicy {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	p := autoscale.AutoscalePolicy{
		ID:                   "asp_test_1",
		Project:              "proj",
		Name:                 "test-policy",
		GroupID:              "grp_1",
		Enabled:              true,
		PolicyType:           autoscale.PolicyTypeTarget,
		MetricName:           autoscale.MetricCPUAvgPercent,
		MetricScope:          autoscale.ScopeGroup,
		TargetValue:          60,
		MinReplicas:          1,
		MaxReplicas:          10,
		ScaleOutStep:         1,
		ScaleInStep:          1,
		ScaleOutCooldownSecs: 60,
		ScaleInCooldownSecs:  300,
		EvalWindowSecs:       300,
		StabWindowSecs:       300,
		ScheduleJSON:         "[]",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	for _, fn := range overrides {
		fn(&p)
	}
	if err := st.Policies.Insert(p); err != nil {
		t.Fatalf("insert policy: %v", err)
	}
	return p
}

// ---- policy store tests -----------------------------------------------------

func TestPolicyCRUD(t *testing.T) {
	st := newTestStore(t)
	p := newPolicy(t, st)

	got, err := st.Policies.Get(p.Name, p.Project)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID mismatch: got %s want %s", got.ID, p.ID)
	}
	if got.GroupID != p.GroupID {
		t.Errorf("GroupID mismatch: got %s want %s", got.GroupID, p.GroupID)
	}

	// List
	all, err := st.Policies.List(p.Project)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("List: got %d policies want 1", len(all))
	}

	// ListEnabled
	enabled, err := st.Policies.ListEnabled()
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}
	if len(enabled) != 1 {
		t.Errorf("ListEnabled: got %d want 1", len(enabled))
	}

	// Update
	p.Enabled = false
	if err := st.Policies.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}
	enabled2, _ := st.Policies.ListEnabled()
	if len(enabled2) != 0 {
		t.Errorf("after disable: ListEnabled should return 0, got %d", len(enabled2))
	}

	// Delete
	if err := st.Policies.Delete(p.Name, p.Project); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = st.Policies.Get(p.Name, p.Project)
	if err != autoscale.ErrNotFound {
		t.Errorf("after delete: expected ErrNotFound, got %v", err)
	}
}

func TestDecisionInsertAndList(t *testing.T) {
	st := newTestStore(t)

	d := autoscale.AutoscaleDecision{
		ID:          "asd_1",
		PolicyID:    "asp_1",
		GroupID:     "grp_1",
		Project:     "proj",
		OldReplicas: 3, NewReplicas: 5, RecommendedReplicas: 5,
		Decision: autoscale.DecisionScaleOut,
		Reason:   "cpu high",
		MetricName: autoscale.MetricCPUAvgPercent, MetricValue: 88, TargetValue: 60,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := st.Decisions.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	list, err := st.Decisions.ListForGroup("grp_1", 10)
	if err != nil {
		t.Fatalf("ListForGroup: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListForGroup: got %d want 1", len(list))
	}
	if list[0].Decision != autoscale.DecisionScaleOut {
		t.Errorf("Decision mismatch: got %s", list[0].Decision)
	}
}

func TestSampleInsertAndAverage(t *testing.T) {
	st := newTestStore(t)

	for i := 0; i < 3; i++ {
		if err := st.Samples.Insert(autoscale.MetricSample{
			ID:         "ms_" + string(rune('0'+i)),
			Scope:      autoscale.ScopeGroup,
			ResourceID: "grp_1",
			MetricName: autoscale.MetricCPUAvgPercent,
			Value:      float64(60 + i*10),
			SampledAt:  time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("Insert sample: %v", err)
		}
	}

	avg, ok := st.Samples.Average(autoscale.ScopeGroup, "grp_1", autoscale.MetricCPUAvgPercent, 60)
	if !ok {
		t.Fatal("Average: expected ok=true")
	}
	// values: 60, 70, 80 → avg = 70
	if avg != 70 {
		t.Errorf("Average: got %.2f want 70", avg)
	}
}

// ---- evaluator tests --------------------------------------------------------

func TestTargetEvaluatorScaleOut(t *testing.T) {
	query := func(_ context.Context, _, _ string) (float64, error) { return 90, nil }
	ev := evaluator.NewTargetEvaluator(query)

	p := autoscale.AutoscalePolicy{
		Enabled:     true,
		PolicyType:  autoscale.PolicyTypeTarget,
		MetricName:  autoscale.MetricCPUAvgPercent,
		TargetValue: 60,
		MinReplicas: 1,
		MaxReplicas: 20,
	}
	rec, err := ev.Evaluate(context.Background(), p, 4)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// ceil(4 * 90 / 60) = ceil(6) = 6
	if rec.Decision != autoscale.DecisionScaleOut {
		t.Errorf("decision: got %s want scale-out", rec.Decision)
	}
	if rec.NewReplicas != 6 {
		t.Errorf("NewReplicas: got %d want 6", rec.NewReplicas)
	}
}

func TestTargetEvaluatorScaleIn(t *testing.T) {
	query := func(_ context.Context, _, _ string) (float64, error) { return 20, nil }
	ev := evaluator.NewTargetEvaluator(query)

	p := autoscale.AutoscalePolicy{
		Enabled:     true,
		MetricName:  autoscale.MetricCPUAvgPercent,
		TargetValue: 60,
		MinReplicas: 2,
		MaxReplicas: 20,
	}
	rec, err := ev.Evaluate(context.Background(), p, 6)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// ceil(6 * 20 / 60) = ceil(2) = 2
	if rec.Decision != autoscale.DecisionScaleIn {
		t.Errorf("decision: got %s want scale-in", rec.Decision)
	}
	if rec.NewReplicas != 2 {
		t.Errorf("NewReplicas: got %d want 2", rec.NewReplicas)
	}
}

func TestTargetEvaluatorHold(t *testing.T) {
	query := func(_ context.Context, _, _ string) (float64, error) { return 60, nil }
	ev := evaluator.NewTargetEvaluator(query)

	p := autoscale.AutoscalePolicy{
		Enabled:     true,
		MetricName:  autoscale.MetricCPUAvgPercent,
		TargetValue: 60,
		MinReplicas: 1,
		MaxReplicas: 10,
	}
	rec, _ := ev.Evaluate(context.Background(), p, 4)
	if rec.Decision != autoscale.DecisionHold {
		t.Errorf("decision: got %s want hold", rec.Decision)
	}
	if rec.NewReplicas != 4 {
		t.Errorf("NewReplicas: got %d want 4", rec.NewReplicas)
	}
}

func TestTargetEvaluatorClampsToMax(t *testing.T) {
	query := func(_ context.Context, _, _ string) (float64, error) { return 200, nil }
	ev := evaluator.NewTargetEvaluator(query)

	p := autoscale.AutoscalePolicy{
		Enabled:     true,
		MetricName:  autoscale.MetricCPUAvgPercent,
		TargetValue: 60,
		MinReplicas: 1,
		MaxReplicas: 5,
	}
	rec, _ := ev.Evaluate(context.Background(), p, 4)
	if rec.NewReplicas != 5 {
		t.Errorf("NewReplicas: got %d want 5 (clamped to max)", rec.NewReplicas)
	}
}

func TestThresholdEvaluatorScaleOut(t *testing.T) {
	query := func(_ context.Context, _, _ string) (float64, error) { return 80, nil }
	ev := evaluator.NewThresholdEvaluator(query)

	p := autoscale.AutoscalePolicy{
		Enabled:           true,
		MetricName:        autoscale.MetricCPUAvgPercent,
		ScaleOutThreshold: 75,
		ScaleInThreshold:  25,
		ScaleOutStep:      2,
		MinReplicas:       1,
		MaxReplicas:       20,
	}
	rec, _ := ev.Evaluate(context.Background(), p, 3)
	if rec.Decision != autoscale.DecisionScaleOut {
		t.Errorf("decision: got %s want scale-out", rec.Decision)
	}
	if rec.NewReplicas != 5 { // 3 + step(2)
		t.Errorf("NewReplicas: got %d want 5", rec.NewReplicas)
	}
}

func TestQueueEvaluatorScaleOut(t *testing.T) {
	depthFn := func(_ context.Context, _ string) (int64, error) { return 200, nil }
	ev := evaluator.NewQueueEvaluator(depthFn)

	p := autoscale.AutoscalePolicy{
		Enabled:     true,
		QueueName:   "jobs",
		TargetValue: 50, // 50 messages per worker
		MinReplicas: 1,
		MaxReplicas: 20,
	}
	rec, _ := ev.Evaluate(context.Background(), p, 2)
	// ceil(200 / 50) = 4
	if rec.Decision != autoscale.DecisionScaleOut {
		t.Errorf("decision: got %s want scale-out", rec.Decision)
	}
	if rec.NewReplicas != 4 {
		t.Errorf("NewReplicas: got %d want 4", rec.NewReplicas)
	}
}

func TestQueueEvaluatorEmptyQueueScalesIn(t *testing.T) {
	depthFn := func(_ context.Context, _ string) (int64, error) { return 0, nil }
	ev := evaluator.NewQueueEvaluator(depthFn)

	p := autoscale.AutoscalePolicy{
		Enabled:     true,
		QueueName:   "jobs",
		TargetValue: 50,
		MinReplicas: 0,
		MaxReplicas: 20,
	}
	rec, _ := ev.Evaluate(context.Background(), p, 5)
	if rec.Decision != autoscale.DecisionScaleIn {
		t.Errorf("decision: got %s want scale-in", rec.Decision)
	}
	if rec.NewReplicas != 0 {
		t.Errorf("NewReplicas: got %d want 0", rec.NewReplicas)
	}
}

// ---- reconciler tests -------------------------------------------------------

type mockPolicyStore struct {
	policies []autoscale.AutoscalePolicy
	lastScale map[string]string
}

func (m *mockPolicyStore) ListEnabled() ([]autoscale.AutoscalePolicy, error) {
	var out []autoscale.AutoscalePolicy
	for _, p := range m.policies {
		if p.Enabled {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *mockPolicyStore) GetByID(id string) (autoscale.AutoscalePolicy, error) {
	for _, p := range m.policies {
		if p.ID == id {
			return p, nil
		}
	}
	return autoscale.AutoscalePolicy{}, autoscale.ErrNotFound
}

func (m *mockPolicyStore) UpdateLastScale(id, decision string) error {
	if m.lastScale == nil {
		m.lastScale = make(map[string]string)
	}
	m.lastScale[id] = decision
	return nil
}

type mockDecisionStore struct {
	decisions []autoscale.AutoscaleDecision
}

func (m *mockDecisionStore) Insert(d autoscale.AutoscaleDecision) error {
	m.decisions = append(m.decisions, d)
	return nil
}

type mockScaler struct {
	current  int
	desired  int
	setErr   error
}

func (m *mockScaler) CurrentReplicas(_ context.Context, _ string) (int, error) {
	return m.current, nil
}

func (m *mockScaler) SetDesiredReplicas(_ context.Context, _ string, desired int) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.desired = desired
	return nil
}

func TestReconcilerScalesOut(t *testing.T) {
	policies := &mockPolicyStore{policies: []autoscale.AutoscalePolicy{{
		ID: "asp_1", Enabled: true, GroupID: "grp_1",
		PolicyType: autoscale.PolicyTypeTarget, MetricName: autoscale.MetricCPUAvgPercent,
		TargetValue: 60, MinReplicas: 1, MaxReplicas: 10,
		ScaleOutCooldownSecs: 0, ScaleInCooldownSecs: 0,
	}}}
	decisions := &mockDecisionStore{}

	// Metric returns 90% CPU.
	query := func(_ context.Context, _, _ string) (float64, error) { return 90, nil }
	resolve := func(pt string) (autoscale.Evaluator, error) {
		return evaluator.NewTargetEvaluator(query), nil
	}

	scaler := &mockScaler{current: 4}
	r := autoscale.NewReconciler(autoscale.ReconcilerStore{
		Policies:  policies,
		Decisions: decisions,
	}, resolve)

	if err := r.Reconcile(context.Background(), scaler); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// ceil(4 * 90/60) = 6
	if scaler.desired != 6 {
		t.Errorf("desired replicas: got %d want 6", scaler.desired)
	}
	if len(decisions.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions.decisions))
	}
	if decisions.decisions[0].Decision != autoscale.DecisionScaleOut {
		t.Errorf("decision: got %s want scale-out", decisions.decisions[0].Decision)
	}
}

func TestReconcilerCooldownBlocks(t *testing.T) {
	lastScale := time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339)
	policies := &mockPolicyStore{policies: []autoscale.AutoscalePolicy{{
		ID: "asp_1", Enabled: true, GroupID: "grp_1",
		PolicyType: autoscale.PolicyTypeTarget, MetricName: autoscale.MetricCPUAvgPercent,
		TargetValue: 60, MinReplicas: 1, MaxReplicas: 10,
		ScaleOutCooldownSecs: 60, // 60s cooldown, only 10s elapsed
		LastScaleAt:          lastScale,
	}}}
	decisions := &mockDecisionStore{}

	query := func(_ context.Context, _, _ string) (float64, error) { return 90, nil }
	resolve := func(pt string) (autoscale.Evaluator, error) {
		return evaluator.NewTargetEvaluator(query), nil
	}

	scaler := &mockScaler{current: 4}
	r := autoscale.NewReconciler(autoscale.ReconcilerStore{
		Policies:  policies,
		Decisions: decisions,
	}, resolve)

	_ = r.Reconcile(context.Background(), scaler)

	// Scaler should NOT have been updated due to cooldown.
	if scaler.desired != 0 {
		t.Errorf("expected no scale during cooldown, got desired=%d", scaler.desired)
	}
	if len(decisions.decisions) == 0 {
		t.Fatal("expected a decision to be recorded")
	}
	if decisions.decisions[0].Decision != autoscale.DecisionBlocked {
		t.Errorf("decision: got %s want blocked", decisions.decisions[0].Decision)
	}
}
