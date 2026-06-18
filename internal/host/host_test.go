package host_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/host"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := host.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

// TestUpsertAndGet verifies that a host can be stored and retrieved.
func TestUpsertAndGet(t *testing.T) {
	db := openTestDB(t)
	s := host.NewStore(db)

	h := host.Host{
		ID:            "host_node1",
		Hostname:      "node1",
		Roles:         []string{"compute"},
		Labels:        map[string]string{"region": "local"},
		OS:            "linux",
		Arch:          "amd64",
		KernelVersion: "6.9.3",
		CPUCount:      8,
		MemoryBytes:   16 * 1024 * 1024 * 1024,
		Addresses:     []string{"192.168.1.10"},
		Status:        host.StatusReady,
	}
	if err := s.Upsert(h); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get("node1")
	if err != nil {
		t.Fatalf("Get by hostname: %v", err)
	}
	if got.CPUCount != h.CPUCount {
		t.Errorf("CPUCount: want %d got %d", h.CPUCount, got.CPUCount)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "compute" {
		t.Errorf("Roles not preserved: %v", got.Roles)
	}
	if got.Labels["region"] != "local" {
		t.Errorf("Labels not preserved: %v", got.Labels)
	}
	if got.LastSeen == "" {
		t.Error("LastSeen should be set after Upsert")
	}
}

// TestUpsertIdempotent verifies that upserting the same host twice updates it.
func TestUpsertIdempotent(t *testing.T) {
	db := openTestDB(t)
	s := host.NewStore(db)

	h := host.Host{ID: "host_x", Hostname: "hostx", OS: "linux", Arch: "arm64", Status: host.StatusReady}
	if err := s.Upsert(h); err != nil {
		t.Fatal(err)
	}
	h.Arch = "amd64" // simulate a re-register with updated info
	if err := s.Upsert(h); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, _ := s.Get("host_x")
	if got.Arch != "amd64" {
		t.Errorf("Arch should be updated on second upsert, got %q", got.Arch)
	}
}

// TestList verifies multiple hosts are returned.
func TestList(t *testing.T) {
	db := openTestDB(t)
	s := host.NewStore(db)

	for i, name := range []string{"alpha", "beta", "gamma"} {
		_ = s.Upsert(host.Host{
			ID:       "host_" + name,
			Hostname: name,
			OS:       "linux",
			Arch:     "amd64",
			Status:   host.StatusReady,
			CPUCount: i + 1,
		})
	}
	hosts, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(hosts) != 3 {
		t.Errorf("expected 3 hosts, got %d", len(hosts))
	}
}

// TestUpdateSeen verifies LastSeen is bumped.
func TestUpdateSeen(t *testing.T) {
	db := openTestDB(t)
	s := host.NewStore(db)

	h := host.Host{ID: "host_z", Hostname: "z", OS: "linux", Arch: "amd64", Status: host.StatusReady}
	_ = s.Upsert(h)
	before, _ := s.Get("z")
	if err := s.UpdateSeen("host_z"); err != nil {
		t.Fatalf("UpdateSeen: %v", err)
	}
	after, _ := s.Get("z")
	// LastSeen should be >= before
	if after.LastSeen < before.LastSeen {
		t.Errorf("LastSeen should advance: before=%s after=%s", before.LastSeen, after.LastSeen)
	}
}

// TestSetLabels verifies label replacement.
func TestSetLabels(t *testing.T) {
	db := openTestDB(t)
	s := host.NewStore(db)

	_ = s.Upsert(host.Host{ID: "host_lbl", Hostname: "lbl", OS: "linux", Arch: "amd64", Status: host.StatusReady})
	labels := map[string]string{"env": "prod", "rack": "A1"}
	if err := s.SetLabels("host_lbl", labels); err != nil {
		t.Fatalf("SetLabels: %v", err)
	}
	got, _ := s.Get("host_lbl")
	if got.Labels["rack"] != "A1" {
		t.Errorf("label not set: %v", got.Labels)
	}
}

// TestInventory verifies Inventory() returns non-empty fields on Linux.
func TestInventory(t *testing.T) {
	h := host.Inventory()
	if h.Hostname == "" {
		t.Error("Inventory: Hostname should not be empty")
	}
	if h.OS == "" {
		t.Error("Inventory: OS should not be empty")
	}
	if h.CPUCount <= 0 {
		t.Errorf("Inventory: CPUCount should be > 0, got %d", h.CPUCount)
	}
}

// TestInitSchemaIdempotent verifies no error on repeated calls.
func TestInitSchemaIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := host.InitSchema(db); err != nil {
		t.Errorf("second InitSchema: %v", err)
	}
}

func newStore(t *testing.T) *host.Store {
	t.Helper()
	return host.NewStore(openTestDB(t))
}

func TestProvisionImageCRUD(t *testing.T) {
	s := newStore(t)

	img, err := s.CreateProvisionImage(host.ProvisionImage{
		Name:     "ubuntu-24.04",
		Version:  "24.04",
		Path:     "/images/ubuntu-24.04.iso",
		Checksum: "abc123",
	})
	if err != nil {
		t.Fatalf("CreateProvisionImage: %v", err)
	}
	if img.ID == "" {
		t.Error("expected non-empty image ID")
	}

	got, err := s.GetProvisionImage("ubuntu-24.04")
	if err != nil {
		t.Fatalf("GetProvisionImage: %v", err)
	}
	if got.Version != "24.04" {
		t.Errorf("version: %q", got.Version)
	}

	imgs, err := s.ListProvisionImages()
	if err != nil {
		t.Fatalf("ListProvisionImages: %v", err)
	}
	if len(imgs) != 1 {
		t.Errorf("expected 1 image, got %d", len(imgs))
	}

	if err := s.DeleteProvisionImage("ubuntu-24.04"); err != nil {
		t.Fatalf("DeleteProvisionImage: %v", err)
	}
	imgs, _ = s.ListProvisionImages()
	if len(imgs) != 0 {
		t.Errorf("expected 0 images after deletion, got %d", len(imgs))
	}
}

func TestProvisionJobLifecycle(t *testing.T) {
	s := newStore(t)

	img, _ := s.CreateProvisionImage(host.ProvisionImage{Name: "debian-12", Version: "12"})

	job, err := s.CreateProvisionJob("host_001", img.ID, "pxe")
	if err != nil {
		t.Fatalf("CreateProvisionJob: %v", err)
	}
	if job.Status != "pending" {
		t.Errorf("initial status: %q", job.Status)
	}
	if job.Method != "pxe" {
		t.Errorf("method: %q", job.Method)
	}

	if err := s.UpdateProvisionJob(job.ID, "running"); err != nil {
		t.Fatalf("UpdateProvisionJob running: %v", err)
	}
	if err := s.UpdateProvisionJob(job.ID, "complete"); err != nil {
		t.Fatalf("UpdateProvisionJob complete: %v", err)
	}

	jobs, err := s.ListProvisionJobs("host_001")
	if err != nil {
		t.Fatalf("ListProvisionJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != "complete" {
		t.Errorf("final status: %q", jobs[0].Status)
	}
	if jobs[0].CompletedAt == "" {
		t.Error("expected CompletedAt to be set")
	}
}
