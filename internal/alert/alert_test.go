package alert_test

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/alert"
	"capper/internal/metrics"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := alert.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *alert.Manager {
	t.Helper()
	return alert.NewManager(alert.NewStore(openDB(t)))
}

func TestCreateAndList(t *testing.T) {
	m := newManager(t)
	r, err := m.Create("high-errors", "proj1", alert.RuleTypeEventCount, "instance.failed", 60, 3, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.Name != "high-errors" {
		t.Errorf("name: %q", r.Name)
	}
	rules, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("List: got %d, want 1", len(rules))
	}
}

func TestDelete(t *testing.T) {
	m := newManager(t)
	r, _ := m.Create("temp-rule", "proj1", alert.RuleTypeEventCount, "instance.stopped", 60, 1, "")
	if err := m.Delete(r.Name, "proj1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	rules, _ := m.List("proj1")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestEvaluate_EventCount_Fires(t *testing.T) {
	m := newManager(t)
	// threshold=2, window=60s — trigger once we have ≥2 matching events
	if _, err := m.Create("fail-alert", "proj1", alert.RuleTypeEventCount, "instance.failed", 60, 2, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	events := []alert.EventRecord{
		{Action: "instance.failed", Timestamp: now},
		{Action: "instance.failed", Timestamp: now},
	}
	firings, err := m.Evaluate("proj1", events, nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(firings) == 0 {
		t.Error("expected alert to fire with 2 matching events at threshold 2")
	}
}

func TestEvaluate_EventCount_NoFire(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("high-threshold", "proj1", alert.RuleTypeEventCount, "instance.failed", 60, 5, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	events := []alert.EventRecord{
		{Action: "instance.failed", Timestamp: now},
	}
	firings, err := m.Evaluate("proj1", events, nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(firings) != 0 {
		t.Errorf("expected no firing with 1 event at threshold 5, got %d", len(firings))
	}
}

func TestEvaluate_MetricThreshold_Fires(t *testing.T) {
	m := newManager(t)
	// Threshold=500 memory_bytes — will fire when instance reports ≥500 bytes
	if _, err := m.Create("mem-alert", "proj1", alert.RuleTypeMetricThreshold, "", 0, 500, "memory_bytes"); err != nil {
		t.Fatal(err)
	}
	instMetrics := []metrics.InstanceMetrics{
		{InstanceID: "inst-1", MemoryBytes: 1024},
	}
	firings, err := m.Evaluate("proj1", nil, instMetrics)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(firings) == 0 {
		t.Error("expected metric alert to fire for memory_bytes=1024 at threshold 500")
	}
}
