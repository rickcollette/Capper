// Package integration contains cross-package end-to-end tests that exercise
// multiple internal packages together in-process using an in-memory SQLite database.
package integration_test

import (
	"database/sql"
	"fmt"
	"testing"

	"capper/internal/billing"
	"capper/internal/org"

	_ "modernc.org/sqlite"
)

// openDB opens a shared in-memory SQLite database and initialises schemas for
// all packages used in the test.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := billing.InitSchema(db); err != nil {
		t.Fatalf("billing.InitSchema: %v", err)
	}
	if err := org.InitSchema(db); err != nil {
		t.Fatalf("org.InitSchema: %v", err)
	}
	return db
}

// TestQuotaLaunchDenyAndRelease verifies the project-level quota lifecycle:
//
//  1. Set a quota of 2 instances on a project.
//  2. Launch instances one by one — each launch records usage and checks quota.
//  3. The third launch must be denied.
//  4. Stopping one instance releases its usage.
//  5. The quota check passes again after release.
func TestQuotaLaunchDenyAndRelease(t *testing.T) {
	db := openDB(t)
	bm := billing.NewManager(billing.NewStore(db))

	project := "proj-quota-launch"
	resource := "instance"

	if err := bm.SetQuota(project, resource, 2); err != nil {
		t.Fatalf("SetQuota: %v", err)
	}

	// Simulate launching two instances — record usage for each.
	launch := func(instanceID string) error {
		if err := bm.CheckQuota(project, resource); err != nil {
			return fmt.Errorf("quota check denied launch of %s: %w", instanceID, err)
		}
		return bm.RecordUsage(project, resource, instanceID, "count", 1)
	}

	if err := launch("inst-1"); err != nil {
		t.Fatalf("launch inst-1: %v", err)
	}
	if err := launch("inst-2"); err != nil {
		t.Fatalf("launch inst-2: %v", err)
	}

	// Third launch must be denied.
	if err := launch("inst-3"); err == nil {
		t.Fatal("expected quota denial for third instance, got nil")
	}

	used, err := bm.CountUsage(project, resource)
	if err != nil {
		t.Fatalf("CountUsage: %v", err)
	}
	if used != 2 {
		t.Errorf("expected 2 used, got %d", used)
	}

	// Stop inst-1 — releases its usage slot.
	if err := bm.ReleaseUsage(project, resource, "inst-1"); err != nil {
		t.Fatalf("ReleaseUsage inst-1: %v", err)
	}

	// Now a new launch should succeed.
	if err := launch("inst-4"); err != nil {
		t.Fatalf("launch after release: %v", err)
	}
}

// TestOrgAccountQuotaLaunchDenial verifies the account-level quota path used
// in the multi-tenant stack:
//
//  1. Create an org and an account under it.
//  2. Set an account-level instance quota of 1 on the account.
//  3. Record one usage — quota is now at the limit.
//  4. CheckAccountQuota must deny further launches.
func TestOrgAccountQuotaLaunchDenial(t *testing.T) {
	db := openDB(t)
	os := org.NewStore(db)
	bm := billing.NewManager(billing.NewStore(db))

	// Create org hierarchy.
	o, err := os.CreateOrg("acme")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	acct, err := os.CreateAccount(o.ID, "engineering")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// Set a quota of 1 instance for this account.
	bm.SetAccountQuota(acct.ID, "instance", 1)

	// First launch: record usage, then check quota.
	if err := bm.RecordUsage(acct.ID, "instance", "inst-e1", "count", 1); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	// Quota should now be exhausted.
	if err := bm.CheckAccountQuota(acct.ID, "instance"); err == nil {
		t.Fatal("expected account quota denial, got nil")
	}

	// Release inst-e1 and confirm the quota clears.
	if err := bm.ReleaseUsage(acct.ID, "instance", "inst-e1"); err != nil {
		t.Fatalf("ReleaseUsage: %v", err)
	}
	if err := bm.CheckAccountQuota(acct.ID, "instance"); err != nil {
		t.Fatalf("expected quota to clear after release, got: %v", err)
	}
}
