package autoscalestore_test

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/autoscale"
	autoscalestore "capper/internal/autoscale/store"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// InitSchema runs ALTER TABLE on compute_groups; create a stub first.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS compute_groups (id TEXT PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("prereq compute_groups: %v", err)
	}
	if err := autoscalestore.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newStore(t *testing.T) *autoscalestore.Store {
	return autoscalestore.New(openDB(t))
}

func makePolicy(name, project, groupID string) autoscale.AutoscalePolicy {
	return autoscale.AutoscalePolicy{
		ID:          "policy-" + name,
		Project:     project,
		Name:        name,
		GroupID:     groupID,
		Enabled:     true,
		PolicyType:  autoscale.PolicyTypeTarget,
		MetricName:  autoscale.MetricCPUAvgPercent,
		MetricScope: autoscale.ScopeGroup,
		TargetValue: 70.0,
		MinReplicas: 1,
		MaxReplicas: 10,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

// ---- PolicyStore ------------------------------------------------------------

func TestPolicyInsertAndGet(t *testing.T) {
	st := newStore(t)
	p := makePolicy("cpu-policy", "proj1", "group1")

	if err := st.Policies.Insert(p); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := st.Policies.Get("cpu-policy", "proj1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.TargetValue != 70.0 {
		t.Errorf("target value: %v", got.TargetValue)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestPolicyList(t *testing.T) {
	st := newStore(t)
	st.Policies.Insert(makePolicy("p1", "proj1", "g1"))
	st.Policies.Insert(makePolicy("p2", "proj1", "g2"))

	list, err := st.Policies.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 policies, got %d", len(list))
	}
}

func TestPolicyDelete(t *testing.T) {
	st := newStore(t)
	st.Policies.Insert(makePolicy("del-pol", "proj1", "g1"))
	if err := st.Policies.Delete("del-pol", "proj1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := st.Policies.List("proj1")
	if len(list) != 0 {
		t.Errorf("expected 0 policies after delete, got %d", len(list))
	}
}

func TestPolicyGetForGroup(t *testing.T) {
	st := newStore(t)
	st.Policies.Insert(makePolicy("p-g1", "proj1", "group1"))
	st.Policies.Insert(makePolicy("p-g2", "proj1", "group2"))

	list, err := st.Policies.ForGroup("group1")
	if err != nil {
		t.Fatalf("GetForGroup: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 policy for group1, got %d", len(list))
	}
}

// ---- DecisionStore ----------------------------------------------------------

func TestDecisionInsertAndList(t *testing.T) {
	st := newStore(t)
	d := autoscale.AutoscaleDecision{
		ID:          "dec-1",
		PolicyID:    "pol-1",
		GroupID:     "group1",
		Project:     "proj1",
		OldReplicas: 2,
		NewReplicas: 4,
		Decision:    autoscale.DecisionScaleOut,
		Reason:      "cpu above threshold",
		MetricName:  autoscale.MetricCPUAvgPercent,
		MetricValue: 85.0,
		TargetValue: 70.0,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := st.Decisions.Insert(d); err != nil {
		t.Fatalf("Insert decision: %v", err)
	}

	list, err := st.Decisions.ListForGroup("group1", 10)
	if err != nil {
		t.Fatalf("ListForGroup: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(list))
	}
	if list[0].Decision != autoscale.DecisionScaleOut {
		t.Errorf("decision: %v", list[0].Decision)
	}
}

// ---- SampleStore ------------------------------------------------------------

func TestSampleInsertAndAverage(t *testing.T) {
	st := newStore(t)

	for i := 0; i < 3; i++ {
		s := autoscale.MetricSample{
			ID:         "s-" + string(rune('0'+i)),
			Project:    "proj1",
			Scope:      autoscale.ScopeGroup,
			ResourceID: "group1",
			MetricName: autoscale.MetricCPUAvgPercent,
			Value:      float64(60 + i*10), // 60, 70, 80
			SampledAt:  time.Now().UTC().Format(time.RFC3339),
		}
		if err := st.Samples.Insert(s); err != nil {
			t.Fatalf("Insert sample %d: %v", i, err)
		}
	}

	avg, ok := st.Samples.Average(autoscale.ScopeGroup, "group1", autoscale.MetricCPUAvgPercent, 300)
	if !ok {
		t.Fatal("expected average to be found")
	}
	if avg != 70.0 {
		t.Errorf("avg: %v (expected 70.0)", avg)
	}
}

func TestSampleLatest(t *testing.T) {
	st := newStore(t)
	st.Samples.Insert(autoscale.MetricSample{
		ID: "s1", Project: "proj1", Scope: autoscale.ScopeGroup,
		ResourceID: "g1", MetricName: "cpu", Value: 42.0,
		SampledAt: time.Now().UTC().Format(time.RFC3339),
	})
	val, ok := st.Samples.Latest(autoscale.ScopeGroup, "g1", "cpu")
	if !ok {
		t.Fatal("expected latest sample found")
	}
	if val != 42.0 {
		t.Errorf("latest: %v", val)
	}
}

func TestSampleAverage_Empty(t *testing.T) {
	st := newStore(t)
	_, ok := st.Samples.Average(autoscale.ScopeGroup, "no-group", "no-metric", 300)
	if ok {
		t.Error("expected false for empty window")
	}
}
