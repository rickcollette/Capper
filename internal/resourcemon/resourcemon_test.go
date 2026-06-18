package resourcemon

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := NewStore(db)
	if err := s.InitSchema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

func TestUpsertAndListResources(t *testing.T) {
	s := newStore(t)

	r1, err := s.UpsertResource(Resource{ResourceType: "instance", Name: "web-1", Project: "prod", Status: "running"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if r1.ID == "" || r1.Health != HealthHealthy {
		t.Errorf("expected derived health healthy, got %q (id=%q)", r1.Health, r1.ID)
	}

	// Upsert same key updates, does not duplicate.
	r1b, err := s.UpsertResource(Resource{ResourceType: "instance", Name: "web-1", Project: "prod", Status: "stopped"})
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if r1b.ID != r1.ID {
		t.Errorf("upsert created a new row: %q != %q", r1b.ID, r1.ID)
	}

	_, _ = s.UpsertResource(Resource{ResourceType: "network", Name: "net-a", Project: "prod", Status: "active"})

	all, err := s.ListResources(ResourceFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 resources, got %d", len(all))
	}

	insts, _ := s.ListResources(ResourceFilter{ResourceType: "instance"})
	if len(insts) != 1 || insts[0].Name != "web-1" {
		t.Errorf("type filter failed: %+v", insts)
	}

	// Soft delete is excluded from list.
	if err := s.MarkResourceDeleted(r1.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	after, _ := s.ListResources(ResourceFilter{ResourceType: "instance"})
	if len(after) != 0 {
		t.Errorf("soft-deleted resource still listed: %+v", after)
	}
}

func TestMetricsIngestQuery(t *testing.T) {
	s := newStore(t)
	for i, v := range []float64{10, 20, 30} {
		_ = i
		if err := s.InsertSample(MetricSample{
			ResourceType: "instance", ResourceID: "i_1", MetricName: "cpu.percent",
			Value: v, Unit: "percent",
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	samples, err := s.QuerySamples(MetricQuery{ResourceType: "instance", ResourceID: "i_1", MetricName: "cpu.percent"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}
	names, _ := s.MetricNames("instance", "i_1")
	if len(names) != 1 || names[0] != "cpu.percent" {
		t.Errorf("metric names: %v", names)
	}
}

func TestDriftDetection(t *testing.T) {
	in := DetectDrift(`{"port":443,"tls":true}`, `{"tls":true,"port":443}`)
	if in.Status != DriftInSync {
		t.Errorf("expected in_sync for key-reordered equal configs, got %q", in.Status)
	}
	drift := DetectDrift(`{"port":443}`, `{"port":80}`)
	if drift.Status != DriftDrifted {
		t.Errorf("expected drifted, got %q", drift.Status)
	}
	unknown := DetectDrift(`{"port":443}`, ``)
	if unknown.Status != DriftUnknown {
		t.Errorf("expected unknown for empty observed, got %q", unknown.Status)
	}
}

func TestConfigVersioningAndReconcile(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)
	res, _ := s.UpsertResource(Resource{ResourceType: "firewall", Name: "fw-1", Project: "prod"})

	if _, err := mgr.PutDesiredConfig(res.ID, `{"defaultPolicy":"deny"}`, "tester"); err != nil {
		t.Fatalf("put desired: %v", err)
	}
	// Report an observed config that diverges → drift.
	_, drift, err := mgr.ReportObservedConfig(res.ID, `{"defaultPolicy":"allow"}`, "agent")
	if err != nil {
		t.Fatalf("report observed: %v", err)
	}
	if drift.Status != DriftDrifted {
		t.Fatalf("expected drift, got %q", drift.Status)
	}
	drifted, _ := s.ListDrifted()
	if len(drifted) != 1 {
		t.Fatalf("expected 1 drifted resource, got %d", len(drifted))
	}

	// Repair clears drift.
	if _, err := s.RepairDrift(res.ID, "tester"); err != nil {
		t.Fatalf("repair: %v", err)
	}
	after, _ := s.ListDrifted()
	if len(after) != 0 {
		t.Errorf("drift not cleared after repair: %d remain", len(after))
	}
}

func TestAlertRuleEvaluation(t *testing.T) {
	s := newStore(t)
	if _, err := s.CreateAlertRule(AlertRule{
		Name: "high-cpu", ResourceType: "instance", MetricName: "cpu.percent",
		Condition: "gt", Threshold: 80, Severity: "warning", Enabled: true,
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Sample below threshold: no alert.
	_ = s.InsertSample(MetricSample{ResourceType: "instance", ResourceID: "i_1", MetricName: "cpu.percent", Value: 50})
	opened, err := s.EvaluateRules("instance", "i_1")
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(opened) != 0 {
		t.Errorf("expected no alert below threshold, got %d", len(opened))
	}

	// Sample above threshold: one alert.
	_ = s.InsertSample(MetricSample{ResourceType: "instance", ResourceID: "i_1", MetricName: "cpu.percent", Value: 95})
	opened, _ = s.EvaluateRules("instance", "i_1")
	if len(opened) != 1 {
		t.Fatalf("expected 1 alert above threshold, got %d", len(opened))
	}

	// Re-evaluation dedupes (still one open alert).
	_, _ = s.EvaluateRules("instance", "i_1")
	all, _ := s.ListAlerts("open", "", 0)
	if len(all) != 1 {
		t.Errorf("expected 1 open alert after dedupe, got %d", len(all))
	}

	// Ack then resolve.
	if err := s.AckAlert(all[0].ID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if err := s.ResolveAlert(all[0].ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if open, _ := s.ListAlerts("open", "", 0); len(open) != 0 {
		t.Errorf("expected 0 open alerts after resolve, got %d", len(open))
	}
}
