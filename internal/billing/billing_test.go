package billing_test

import (
	"database/sql"
	"testing"

	"capper/internal/billing"

	_ "modernc.org/sqlite"
)

func newManager(t *testing.T, db *sql.DB) *billing.Manager {
	t.Helper()
	if err := billing.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return billing.NewManager(billing.NewStore(db))
}

func TestGovernancePoliciesPersistInSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	m1 := newManager(t, db)
	m1.AddGovernancePolicy("deny-prod-delete", "default", "instance", "delete", "deny", "label.env=prod", 10)

	m2 := billing.NewManager(billing.NewStore(db))
	allowed, rule := m2.EvaluateGovernance("default", "instance", "delete", map[string]string{"env": "prod"})
	if allowed {
		t.Fatal("expected persisted governance deny rule to apply")
	}
	if rule != "deny-prod-delete" {
		t.Fatalf("matched rule = %q", rule)
	}
	policies := m2.ListGovernancePolicies("default")
	if len(policies) != 1 {
		t.Fatalf("expected one persisted policy, got %d", len(policies))
	}
}

func TestAccountQuotasPersistInSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	m1 := newManager(t, db)
	m1.SetAccountQuota("acct-1", "instance", 1)
	if err := m1.RecordUsage("acct-1", "instance", "inst-1", "count", 1); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	m2 := billing.NewManager(billing.NewStore(db))
	if err := m2.CheckAccountQuota("acct-1", "instance"); err == nil {
		t.Fatal("expected persisted account quota to deny additional usage")
	}
}

func TestProjectQuotaAllowsAndDenies(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	m := newManager(t, db)
	if err := m.SetQuota("default", "instance", 2); err != nil {
		t.Fatalf("SetQuota: %v", err)
	}
	if err := m.CheckQuota("default", "instance"); err != nil {
		t.Fatalf("quota should allow initial usage: %v", err)
	}
	_ = m.RecordUsage("default", "instance", "i1", "count", 1)
	if err := m.CheckQuota("default", "instance"); err != nil {
		t.Fatalf("quota should allow second launch: %v", err)
	}
	_ = m.RecordUsage("default", "instance", "i2", "count", 1)
	if err := m.CheckQuota("default", "instance"); err == nil {
		t.Fatal("expected quota denial at limit")
	}
}

func TestReleaseUsageBalancesResourceUsage(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	m := newManager(t, db)
	_ = m.RecordUsage("default", "instance", "i1", "count", 1)
	if used, _ := m.CountUsage("default", "instance"); used != 1 {
		t.Fatalf("expected usage 1 before release, got %d", used)
	}
	if err := m.ReleaseUsage("default", "instance", "i1"); err != nil {
		t.Fatalf("ReleaseUsage: %v", err)
	}
	if used, _ := m.CountUsage("default", "instance"); used != 0 {
		t.Fatalf("expected usage 0 after release, got %d", used)
	}
	if err := m.ReleaseUsage("default", "instance", "i1"); err != nil {
		t.Fatalf("second ReleaseUsage should be idempotent: %v", err)
	}
	if used, _ := m.CountUsage("default", "instance"); used != 0 {
		t.Fatalf("expected usage to remain 0, got %d", used)
	}
}
