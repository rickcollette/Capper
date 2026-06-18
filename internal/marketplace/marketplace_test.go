package marketplace

import (
	"archive/tar"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestManager(t *testing.T) *Manager {
	return NewManager(openTestDB(t))
}

func insertListing(t *testing.T, m *Manager, name string) MarketplaceListing {
	t.Helper()
	l := MarketplaceListing{Name: name, Version: "1.0", Description: "test", Status: StatusPending}
	if err := m.Insert(l); err != nil {
		t.Fatalf("Insert %s: %v", name, err)
	}
	list, _ := m.List()
	for _, item := range list {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("inserted listing %q not found in List()", name)
	return MarketplaceListing{}
}

func insertArtifactListing(t *testing.T, m *Manager, network bool) MarketplaceListing {
	t.Helper()
	path := writeTestCap(t, network, "")
	digest := fileDigestForTest(t, path)
	l := MarketplaceListing{Name: path, Version: "1.0", Description: "test", Status: StatusPending, Digest: digest}
	if err := m.Insert(l); err != nil {
		t.Fatalf("Insert artifact listing: %v", err)
	}
	list, _ := m.List()
	for _, item := range list {
		if item.Name == path {
			return item
		}
	}
	t.Fatal("artifact listing not found")
	return MarketplaceListing{}
}

func TestRunRuntimeScanBlocksMissingArtifact(t *testing.T) {
	m := newTestManager(t)
	l := insertListing(t, m, "test-app")

	result, err := m.RunRuntimeScan(l.ID, 30)
	if err != nil {
		t.Fatalf("RunRuntimeScan: %v", err)
	}
	if result.ListingID != l.ID {
		t.Errorf("ListingID = %q want %q", result.ListingID, l.ID)
	}
	if result.Verdict != "blocked" {
		t.Errorf("Verdict = %q want %q", result.Verdict, "blocked")
	}
	if result.Duration != "30s" {
		t.Errorf("Duration = %q want %q", result.Duration, "30s")
	}
}

func TestRunRuntimeScanClassifiesNetworkManifest(t *testing.T) {
	m := newTestManager(t)
	l := insertArtifactListing(t, m, true)
	result, err := m.RunRuntimeScan(l.ID, 30)
	if err != nil {
		t.Fatalf("RunRuntimeScan: %v", err)
	}
	if result.Verdict != "suspicious" {
		t.Fatalf("expected suspicious verdict for network-enabled manifest, got %q", result.Verdict)
	}
	if len(result.NetworkAttempts) == 0 {
		t.Fatal("expected network observation")
	}
}

// TestRunRuntimeScanUnknownID returns an error for a missing listing.
func TestRunRuntimeScanUnknownID(t *testing.T) {
	m := newTestManager(t)
	_, err := m.RunRuntimeScan("no-such-id", 5)
	if err == nil {
		t.Error("expected error for unknown listing ID")
	}
}

// TestRunRuntimeScanUpdatesStatus verifies that a runtime scan sets scan_status.
func TestRunRuntimeScanUpdatesStatus(t *testing.T) {
	m := newTestManager(t)
	l := insertArtifactListing(t, m, false)
	m.RunRuntimeScan(l.ID, 10)

	var status string
	m.db.QueryRow(`SELECT scan_status FROM marketplace_listings WHERE id=?`, l.ID).Scan(&status)
	if status != "runtime-scan-clean" {
		t.Errorf("scan_status = %q want %q", status, "runtime-scan-clean")
	}
}

func TestRunStaticScansFailsMissingArtifact(t *testing.T) {
	m := newTestManager(t)
	l := insertListing(t, m, "static-app")

	results, err := m.RunStaticScans(l.ID)
	if err != nil {
		t.Fatalf("RunStaticScans: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one static scan result")
	}
	for _, r := range results {
		if r.Status != "fail" {
			t.Fatalf("expected missing artifact scan to fail, got %+v", results)
		}
	}
}

func TestRunStaticScansReadsArtifact(t *testing.T) {
	m := newTestManager(t)
	l := insertArtifactListing(t, m, false)
	results, err := m.RunStaticScans(l.ID)
	if err != nil {
		t.Fatalf("RunStaticScans: %v", err)
	}
	seen := map[string]ScanResult{}
	for _, r := range results {
		seen[r.Type] = r
	}
	for _, typ := range []string{"digest", "signature", "sbom", "secrets", "policy"} {
		if seen[typ].Status != "pass" {
			t.Fatalf("%s scan did not pass: %+v", typ, seen[typ])
		}
	}
	if seen["vuln"].Status != "pass" && seen["vuln"].Status != "warn" && seen["vuln"].Status != "fail" {
		t.Fatalf("unexpected vulnerability scan status: %+v", seen["vuln"])
	}
}

// TestApproveRejectWorkflow exercises the approval state machine.
func TestApproveRejectWorkflow(t *testing.T) {
	m := newTestManager(t)
	l := insertListing(t, m, "review-me")

	if err := m.Approve(l.ID, "LGTM"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	got, _ := m.Get(l.ID)
	if got.Status != StatusApproved {
		t.Errorf("status after Approve = %q want %q", got.Status, StatusApproved)
	}

	l2 := insertListing(t, m, "reject-me")
	if err := m.Reject(l2.ID, "policy violation"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	got2, _ := m.Get(l2.ID)
	if got2.Status != StatusRejected {
		t.Errorf("status after Reject = %q want %q", got2.Status, StatusRejected)
	}
}

func writeTestCap(t *testing.T, network bool, secret string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.cap")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create cap: %v", err)
	}
	tw := tar.NewWriter(f)
	entries := map[string]string{
		"capsule.json":                `{"capsuleVersion":"0.1","name":"app","version":"1","entrypoint":["/bin/sh"],"network":{"enabled":` + boolJSON(network) + `}}`,
		"signature.json":              `{"algorithm":"ed25519","digest":"sha256:test","value":"00"}`,
		"attestations/sbom.spdx.json": `{"spdxVersion":"SPDX-2.3"}`,
		"rootfs/etc/app.conf":         secret,
	}
	for name, body := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cap: %v", err)
	}
	return path
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func fileDigestForTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read digest file: %v", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
