package observability_test

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/observability"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := observability.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

func newStore(t *testing.T) *observability.Store {
	return observability.NewStore(openDB(t))
}

func TestInitSchemaIdempotent(t *testing.T) {
	db := openDB(t)
	if err := observability.InitSchema(db); err != nil {
		t.Errorf("second InitSchema: %v", err)
	}
}

// ---- log tests --------------------------------------------------------------

func TestAppendAndQueryByResource(t *testing.T) {
	s := newStore(t)

	for i, msg := range []string{"started", "ready", "error"} {
		level := observability.LevelInfo
		if i == 2 {
			level = observability.LevelError
		}
		if err := s.AppendLog(observability.LogEntry{
			Resource: "instance/web01",
			Service:  "compute",
			Level:    level,
			Message:  msg,
		}); err != nil {
			t.Fatalf("AppendLog: %v", err)
		}
	}

	logs, err := s.LogsByResource("instance/web01", 10)
	if err != nil {
		t.Fatalf("LogsByResource: %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 log entries, got %d", len(logs))
	}
}

func TestLogsByService(t *testing.T) {
	s := newStore(t)

	for _, svc := range []string{"compute", "compute", "dns"} {
		_ = s.AppendLog(observability.LogEntry{Resource: "x", Service: svc, Message: "msg"})
	}

	logs, err := s.LogsByService("compute", 10)
	if err != nil {
		t.Fatalf("LogsByService: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 compute logs, got %d", len(logs))
	}
}

func TestLogsByLabel(t *testing.T) {
	s := newStore(t)

	_ = s.AppendLog(observability.LogEntry{
		Resource: "instance/api01",
		Service:  "compute",
		Message:  "deployed",
		Labels:   map[string]string{"env": "prod"},
	})
	_ = s.AppendLog(observability.LogEntry{
		Resource: "instance/dev01",
		Service:  "compute",
		Message:  "started",
		Labels:   map[string]string{"env": "dev"},
	})

	logs, err := s.LogsByLabel("env", "prod", 10)
	if err != nil {
		t.Fatalf("LogsByLabel: %v", err)
	}
	if len(logs) != 1 || logs[0].Resource != "instance/api01" {
		t.Errorf("expected api01 log, got %+v", logs)
	}
}

func TestExportJSONL(t *testing.T) {
	s := newStore(t)
	for _, msg := range []string{"a", "b", "c"} {
		_ = s.AppendLog(observability.LogEntry{Resource: "instance/db01", Service: "db", Message: msg})
	}

	out, err := s.ExportJSONL("instance/db01")
	if err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 JSONL lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], `"message"`) {
		t.Errorf("expected JSON object in JSONL line, got: %s", lines[0])
	}
}

// ---- alert rule tests -------------------------------------------------------

func TestAlertRuleLifecycle(t *testing.T) {
	s := newStore(t)

	rule, err := s.CreateAlertRule(observability.AlertRule{
		Name:      "high-error-rate",
		Condition: "error_rate > 0.05",
		Severity:  "high",
	})
	if err != nil {
		t.Fatalf("CreateAlertRule: %v", err)
	}
	if !rule.Enabled {
		t.Error("new alert rule should be enabled")
	}

	rules, err := s.ListAlertRules()
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	if err := s.EnableAlertRule(rule.ID, false); err != nil {
		t.Fatalf("EnableAlertRule: %v", err)
	}
	rules, _ = s.ListAlertRules()
	if rules[0].Enabled {
		t.Error("rule should be disabled")
	}

	if err := s.DeleteAlertRule(rule.ID); err != nil {
		t.Fatalf("DeleteAlertRule: %v", err)
	}
	rules, _ = s.ListAlertRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after deletion, got %d", len(rules))
	}
}

func TestAlertEventFireAndResolve(t *testing.T) {
	s := newStore(t)

	event, err := s.FireAlert("ar_1", "high-error-rate", "error rate exceeded 5%")
	if err != nil {
		t.Fatalf("FireAlert: %v", err)
	}
	if event.State != observability.AlertStateFiring {
		t.Errorf("expected firing, got %q", event.State)
	}

	firing, err := s.ListAlertEvents(observability.AlertStateFiring)
	if err != nil {
		t.Fatalf("ListAlertEvents: %v", err)
	}
	if len(firing) != 1 {
		t.Errorf("expected 1 firing event, got %d", len(firing))
	}

	if err := s.ResolveAlert(event.ID); err != nil {
		t.Fatalf("ResolveAlert: %v", err)
	}
	firing, _ = s.ListAlertEvents(observability.AlertStateFiring)
	if len(firing) != 0 {
		t.Errorf("expected 0 firing events after resolve, got %d", len(firing))
	}

	resolved, err := s.ListAlertEvents(observability.AlertStateResolved)
	if err != nil {
		t.Fatalf("ListAlertEvents resolved: %v", err)
	}
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved event, got %d", len(resolved))
	}
	if resolved[0].ResolvedAt == "" {
		t.Error("resolved event should have non-empty resolved_at")
	}
}
