package csdserver

import (
	"context"
	"testing"
)

func TestSnapshotCreateAndList(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 100 * 1024 * 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "data.bin", ClientID: "c1"})

	// Write some data so we have extents.
	payload := make([]byte, 32*1024)
	for i := range payload {
		payload[i] = byte(i % 199)
	}
	srv.Extents.Write(ctx, WriteReq{VolumeID: v.ID, InodeID: f.ID, Offset: 0, Data: payload, ClientID: "c1"})

	snap, err := srv.Snapshots.Create(ctx, v.ID, "snap-v1")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if snap.ID == "" {
		t.Fatal("no snapshot ID")
	}
	if snap.Status != "available" {
		t.Fatalf("status: %s", snap.Status)
	}
	if snap.SizeBytes <= 0 {
		t.Fatalf("snapshot size should reflect extent data, got %d", snap.SizeBytes)
	}

	snaps, err := srv.Snapshots.List(ctx, v.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 || snaps[0].Name != "snap-v1" {
		t.Fatalf("unexpected snapshots: %v", snaps)
	}
}

func TestSnapshotDelete(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 100 * 1024 * 1024})
	srv.Metadata.EnsureRoot(ctx, v.ID)

	snap, _ := srv.Snapshots.Create(ctx, v.ID, "snap-to-delete")
	if err := srv.Snapshots.Delete(ctx, v.ID, snap.Name); err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
	snaps, _ := srv.Snapshots.List(ctx, v.ID)
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after delete, got %d", len(snaps))
	}
}

func TestSnapshotDuplicateNameFails(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	srv.Metadata.EnsureRoot(ctx, v.ID)

	if _, err := srv.Snapshots.Create(ctx, v.ID, "snap1"); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if _, err := srv.Snapshots.Create(ctx, v.ID, "snap1"); err == nil {
		t.Fatal("expected error for duplicate snapshot name")
	}
}

func TestSnapshotExtentRefCounts(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 100 * 1024 * 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "data.bin", ClientID: "c1"})

	// Write large data to create extents.
	payload := make([]byte, 64*1024)
	srv.Extents.Write(ctx, WriteReq{VolumeID: v.ID, InodeID: f.ID, Offset: 0, Data: payload, ClientID: "c1"})

	// Get initial extent count.
	extentsBefore, _ := srv.Store.Extents.ForVolume(v.ID)

	// Create snapshot — should bump ref counts.
	snap, err := srv.Snapshots.Create(ctx, v.ID, "snap-ref")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	extentsAfter, _ := srv.Store.Extents.ForVolume(v.ID)
	if len(extentsAfter) == 0 {
		t.Fatal("expected extents after snapshot")
	}
	if len(extentsBefore) > 0 {
		// All extents should have ref_count bumped to at least 2.
		for _, e := range extentsAfter {
			if e.RefCount < 2 {
				t.Fatalf("extent %s has ref_count %d, expected >= 2 after snapshot", e.ID, e.RefCount)
			}
		}
	}

	// Delete snapshot — ref counts should drop back.
	if err := srv.Snapshots.Delete(ctx, v.ID, snap.Name); err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
}
