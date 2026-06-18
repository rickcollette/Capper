package csdstore_test

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := csdstore.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newStore(t *testing.T) *csdstore.Store {
	return csdstore.New(openDB(t))
}

func makeVolume(id, name, project string) csd.Volume {
	now := time.Now().UTC().Format(time.RFC3339)
	return csd.Volume{
		ID:        id,
		Project:   project,
		Name:      name,
		Mode:      "rw",
		SizeBytes: 10 * 1024 * 1024 * 1024, // 10 GiB
		Status:    "available",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ---- Volume -----------------------------------------------------------------

func TestVolumeInsertAndGet(t *testing.T) {
	st := newStore(t)
	v := makeVolume("vol-1", "data", "proj1")

	if err := st.Volumes.Insert(v); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := st.Volumes.Get("vol-1", "proj1")
	if err != nil {
		t.Fatalf("Get by ID: %v", err)
	}
	if got.SizeBytes != v.SizeBytes {
		t.Errorf("size bytes: %d", got.SizeBytes)
	}

	got2, err := st.Volumes.Get("data", "proj1")
	if err != nil {
		t.Fatalf("Get by name: %v", err)
	}
	if got2.ID != "vol-1" {
		t.Errorf("id by name: %q", got2.ID)
	}
}

func TestVolumeList(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("v1", "vol-a", "proj1"))
	st.Volumes.Insert(makeVolume("v2", "vol-b", "proj1"))
	st.Volumes.Insert(makeVolume("v3", "other", "proj2"))

	list, err := st.Volumes.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 volumes for proj1, got %d", len(list))
	}
}

func TestVolumeUpdateStatus(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("vol-s", "disk", "proj1"))
	if err := st.Volumes.UpdateStatus("vol-s", "attaching"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ := st.Volumes.Get("vol-s", "proj1")
	if got.Status != "attaching" {
		t.Errorf("status: %q", got.Status)
	}
}

func TestVolumeUpdateUsed(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("vol-u", "disk", "proj1"))
	if err := st.Volumes.UpdateUsed("vol-u", 512*1024); err != nil {
		t.Fatalf("UpdateUsed: %v", err)
	}
	got, _ := st.Volumes.Get("vol-u", "proj1")
	if got.UsedBytes != 512*1024 {
		t.Errorf("used bytes: %d", got.UsedBytes)
	}
}

func TestVolumeDelete(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("del-v", "del-disk", "proj1"))
	if err := st.Volumes.Delete("del-v"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := st.Volumes.List("proj1")
	if len(list) != 0 {
		t.Errorf("expected 0 volumes after delete, got %d", len(list))
	}
}

// ---- Snapshot ---------------------------------------------------------------

func TestSnapshotInsertAndGet(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("vol-1", "disk", "proj1"))

	snap := csd.Snapshot{
		ID:        "snap-1",
		VolumeID:  "vol-1",
		Name:      "daily-backup",
		Status:    "creating",
		SizeBytes: 1024 * 1024,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := st.Snapshots.Insert(snap); err != nil {
		t.Fatalf("Insert snapshot: %v", err)
	}

	got, err := st.Snapshots.Get("vol-1", "snap-1")
	if err != nil {
		t.Fatalf("Get snapshot by ID: %v", err)
	}
	if got.Name != "daily-backup" {
		t.Errorf("name: %q", got.Name)
	}

	got2, err := st.Snapshots.Get("vol-1", "daily-backup")
	if err != nil {
		t.Fatalf("Get snapshot by name: %v", err)
	}
	if got2.ID != "snap-1" {
		t.Errorf("id: %q", got2.ID)
	}
}

func TestSnapshotList(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("vol-1", "disk", "proj1"))
	st.Snapshots.Insert(csd.Snapshot{ID: "s1", VolumeID: "vol-1", Name: "snap-a", Status: "ready", CreatedAt: time.Now().UTC().Format(time.RFC3339)})
	st.Snapshots.Insert(csd.Snapshot{ID: "s2", VolumeID: "vol-1", Name: "snap-b", Status: "ready", CreatedAt: time.Now().UTC().Format(time.RFC3339)})

	list, err := st.Snapshots.List("vol-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(list))
	}
}

func TestSnapshotUpdateStatus(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("vol-1", "disk", "proj1"))
	st.Snapshots.Insert(csd.Snapshot{ID: "s1", VolumeID: "vol-1", Name: "s", Status: "creating", CreatedAt: time.Now().UTC().Format(time.RFC3339)})

	if err := st.Snapshots.UpdateStatus("s1", "ready"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ := st.Snapshots.Get("vol-1", "s1")
	if got.Status != "ready" {
		t.Errorf("status: %q", got.Status)
	}
}

func TestSnapshotDelete(t *testing.T) {
	st := newStore(t)
	st.Volumes.Insert(makeVolume("vol-1", "disk", "proj1"))
	st.Snapshots.Insert(csd.Snapshot{ID: "s-del", VolumeID: "vol-1", Name: "del-snap", Status: "ready", CreatedAt: time.Now().UTC().Format(time.RFC3339)})

	if err := st.Snapshots.Delete("s-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := st.Snapshots.List("vol-1")
	if len(list) != 0 {
		t.Errorf("expected 0 snapshots after delete, got %d", len(list))
	}
}
