package csdserver

import (
	"context"
	"database/sql"
	"testing"

	csdbackend "capper/internal/csd/backend"
	csdstore "capper/internal/csd/store"

	_ "modernc.org/sqlite"
)

// newTestServer returns a fully wired CSD server backed by an in-memory SQLite
// DB and a temp-dir local backend.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := csdstore.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	store := csdstore.New(db)

	backend, err := csdbackend.NewLocalBackend(t.TempDir())
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}
	return NewServer(store, backend)
}

// ---- Phase 1: volume control plane ------------------------------------------

func TestVolumeCreateListGetDelete(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, err := srv.Volumes.Create(ctx, CreateVolumeOpts{
		Project:   "p1",
		Name:      "test-vol",
		Mode:      "shared-fs",
		SizeBytes: 100 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if v.ID == "" {
		t.Fatal("no ID assigned")
	}
	if v.Status != "available" {
		t.Fatalf("want status=available got %s", v.Status)
	}

	vols, err := srv.Volumes.List(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(vols) != 1 || vols[0].Name != "test-vol" {
		t.Fatalf("unexpected list result: %v", vols)
	}

	got, err := srv.Volumes.Get(ctx, "test-vol", "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != v.ID {
		t.Fatalf("id mismatch: %s != %s", got.ID, v.ID)
	}

	if err := srv.Volumes.Delete(ctx, "test-vol", "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := srv.Volumes.Get(ctx, "test-vol", "p1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestVolumeCreateDefaults(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, err := srv.Volumes.Create(ctx, CreateVolumeOpts{
		Name:      "defaults-vol",
		SizeBytes: 1024,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if v.Mode != "shared-fs" {
		t.Fatalf("default mode: %s", v.Mode)
	}
	if v.ReplicaCount != 1 {
		t.Fatalf("default replicas: %d", v.ReplicaCount)
	}
	if v.StorageClass != "local" {
		t.Fatalf("default storage class: %s", v.StorageClass)
	}
	if v.Epoch != 1 {
		t.Fatalf("epoch should start at 1, got %d", v.Epoch)
	}
}

func TestVolumeCreateValidation(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	if _, err := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "", SizeBytes: 1024}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "v", SizeBytes: 0}); err == nil {
		t.Fatal("expected error for zero size")
	}
	if _, err := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "v", SizeBytes: 1024, Mode: "bad-mode"}); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestVolumeDeleteWithActiveAttachmentFails(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "busy", SizeBytes: 1024})
	_, err := srv.Volumes.Attach(ctx, AttachOpts{
		VolumeID:   v.ID,
		InstanceID: "inst-1",
		MountPath:  "/mnt/x",
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := srv.Volumes.Delete(ctx, "busy", ""); err == nil {
		t.Fatal("expected error deleting volume with active attachment")
	}
}

// ---- Phase 1: attachments ---------------------------------------------------

func TestAttachDetach(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})

	a, err := srv.Volumes.Attach(ctx, AttachOpts{
		VolumeID:   v.ID,
		InstanceID: "inst-1",
		MountPath:  "/mnt/media",
		AccessMode: "rw",
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if a.ID == "" {
		t.Fatal("no attachment ID")
	}

	atts, err := srv.Volumes.ListAttachments(ctx, "vol")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(atts))
	}

	if err := srv.Volumes.Detach(ctx, "vol", "inst-1"); err != nil {
		t.Fatalf("detach: %v", err)
	}
	atts, _ = srv.Volumes.ListAttachments(ctx, "vol")
	if len(atts) != 0 {
		t.Fatalf("expected 0 attachments after detach, got %d", len(atts))
	}
}

func TestSingleWriterOnlyOneRW(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "sw-vol", SizeBytes: 1024, Mode: "single-writer"})
	_, err := srv.Volumes.Attach(ctx, AttachOpts{VolumeID: v.ID, InstanceID: "inst-1", MountPath: "/m", AccessMode: "rw"})
	if err != nil {
		t.Fatalf("first attach: %v", err)
	}
	_, err = srv.Volumes.Attach(ctx, AttachOpts{VolumeID: v.ID, InstanceID: "inst-2", MountPath: "/m", AccessMode: "rw"})
	if err == nil {
		t.Fatal("expected error for second rw attachment on single-writer volume")
	}
}

func TestMultipleInstancesSharedFS(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "shared", SizeBytes: 1024})
	for i, id := range []string{"inst-a", "inst-b", "inst-c"} {
		mp := "/mnt/shared"
		mode := "rw"
		if i == 2 {
			mode = "ro"
		}
		if _, err := srv.Volumes.Attach(ctx, AttachOpts{VolumeID: v.ID, InstanceID: id, MountPath: mp, AccessMode: mode}); err != nil {
			t.Fatalf("attach %s: %v", id, err)
		}
	}
	atts, _ := srv.Volumes.ListAttachments(ctx, "shared")
	if len(atts) != 3 {
		t.Fatalf("expected 3 attachments, got %d", len(atts))
	}
}

// ---- Phase 2: metadata manager ----------------------------------------------

func TestMetadataEnsureRoot(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})

	root, err := srv.Metadata.EnsureRoot(ctx, v.ID)
	if err != nil {
		t.Fatalf("ensure root: %v", err)
	}
	if root.ID != "root-"+v.ID {
		t.Fatalf("unexpected root ID: %s", root.ID)
	}
	if root.Type != "directory" {
		t.Fatalf("root type: %s", root.Type)
	}

	// Idempotent.
	root2, err := srv.Metadata.EnsureRoot(ctx, v.ID)
	if err != nil {
		t.Fatalf("second ensure root: %v", err)
	}
	if root2.ID != root.ID {
		t.Fatal("second call should return same root")
	}
}

func TestMetadataCreateFile(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)

	f, err := srv.Metadata.Create(ctx, CreateReq{
		VolumeID: v.ID,
		ParentID: root.ID,
		Name:     "hello.txt",
		ModeBits: 0o644,
		ClientID: "c1",
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if f.Name != "hello.txt" {
		t.Fatalf("name: %s", f.Name)
	}
	if f.Type != "file" {
		t.Fatalf("type: %s", f.Type)
	}

	got, err := srv.Metadata.Lookup(ctx, v.ID, root.ID, "hello.txt")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ID != f.ID {
		t.Fatalf("lookup ID mismatch")
	}
}

func TestMetadataMkdir(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)

	dir, err := srv.Metadata.Mkdir(ctx, CreateReq{
		VolumeID: v.ID,
		ParentID: root.ID,
		Name:     "subdir",
		ModeBits: 0o755,
		ClientID: "c1",
	})
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if dir.Type != "directory" {
		t.Fatalf("type: %s", dir.Type)
	}
}

func TestMetadataReaddir(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)

	names := []string{"a.txt", "b.txt", "c.txt"}
	for _, name := range names {
		srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: name, ClientID: "c1"})
	}

	children, err := srv.Metadata.Readdir(ctx, v.ID, root.ID)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}

func TestMetadataRename(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)

	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "old.txt", ClientID: "c1"})

	err := srv.Metadata.Rename(ctx, RenameReq{
		VolumeID:    v.ID,
		InodeID:     f.ID,
		OldParentID: root.ID, OldName: "old.txt",
		NewParentID: root.ID, NewName: "new.txt",
		ClientID: "c1",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}

	if _, err := srv.Metadata.Lookup(ctx, v.ID, root.ID, "old.txt"); err == nil {
		t.Fatal("old name should not exist after rename")
	}
	got, err := srv.Metadata.Lookup(ctx, v.ID, root.ID, "new.txt")
	if err != nil {
		t.Fatalf("lookup new name: %v", err)
	}
	if got.ID != f.ID {
		t.Fatal("renamed inode ID mismatch")
	}
}

func TestMetadataUnlink(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)

	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "gone.txt", ClientID: "c1"})

	err := srv.Metadata.Unlink(ctx, UnlinkReq{
		VolumeID: v.ID, InodeID: f.ID, ParentID: root.ID, Name: "gone.txt", ClientID: "c1",
	})
	if err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, err := srv.Metadata.Lookup(ctx, v.ID, root.ID, "gone.txt"); err == nil {
		t.Fatal("file should not exist after unlink")
	}
}

func TestMetadataTruncate(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	if err := srv.Metadata.Truncate(ctx, TruncateReq{
		VolumeID: v.ID, InodeID: f.ID, NewSize: 512, ClientID: "c1",
	}); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	got, _ := srv.Metadata.Getattr(ctx, f.ID)
	if got.SizeBytes != 512 {
		t.Fatalf("size after truncate: %d", got.SizeBytes)
	}
}

// ---- Phase 2: extent manager ------------------------------------------------

func TestExtentWriteReadInline(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024 * 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "small.txt", ClientID: "c1"})

	payload := []byte("hello from inline")
	if err := srv.Extents.Write(ctx, WriteReq{
		VolumeID: v.ID, InodeID: f.ID, Offset: 0, Data: payload, ClientID: "c1",
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := srv.Extents.Read(ctx, ReadReq{VolumeID: v.ID, InodeID: f.ID, Offset: 0, Length: len(payload)})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("read mismatch: %q != %q", got, payload)
	}
}

func TestExtentWriteReadLarge(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 100 * 1024 * 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "large.bin", ClientID: "c1"})

	// Write more than InlineMaxBytes (16 KiB) to force extent path.
	payload := make([]byte, 32*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := srv.Extents.Write(ctx, WriteReq{
		VolumeID: v.ID, InodeID: f.ID, Offset: 0, Data: payload, ClientID: "c1",
	}); err != nil {
		t.Fatalf("write large: %v", err)
	}

	got, err := srv.Extents.Read(ctx, ReadReq{VolumeID: v.ID, InodeID: f.ID, Offset: 0, Length: len(payload)})
	if err != nil {
		t.Fatalf("read large: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("read length %d != %d", len(got), len(payload))
	}
	for i, b := range got {
		if b != payload[i] {
			t.Fatalf("byte mismatch at %d: %d != %d", i, b, payload[i])
		}
	}
}

func TestExtentPartialRead(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024 * 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.bin", ClientID: "c1"})

	payload := []byte("0123456789")
	srv.Extents.Write(ctx, WriteReq{VolumeID: v.ID, InodeID: f.ID, Offset: 0, Data: payload, ClientID: "c1"})

	got, err := srv.Extents.Read(ctx, ReadReq{VolumeID: v.ID, InodeID: f.ID, Offset: 2, Length: 5})
	if err != nil {
		t.Fatalf("partial read: %v", err)
	}
	if string(got) != "23456" {
		t.Fatalf("partial read: %q", got)
	}
}

func TestExtentInodeSizeUpdated(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024 * 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	payload := []byte("hello")
	srv.Extents.Write(ctx, WriteReq{VolumeID: v.ID, InodeID: f.ID, Offset: 0, Data: payload, ClientID: "c1"})

	inode, err := srv.Metadata.Getattr(ctx, f.ID)
	if err != nil {
		t.Fatalf("getattr: %v", err)
	}
	if inode.SizeBytes != int64(len(payload)) {
		t.Fatalf("inode size %d != %d", inode.SizeBytes, len(payload))
	}
}

// ---- Phase 2: journal -------------------------------------------------------

func TestJournalReplay(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)

	// Simulate a crash mid-create: append a pending journal entry but do NOT
	// insert the inode (as if the process died between journal append and inode write).
	inodeID := "test-replay-inode-id"
	seq, err := srv.Journal.Append(ctx, v.ID, "c1", "", "create", inodeID, map[string]any{
		"name": "replay.txt", "parent_id": root.ID, "mode_bits": float64(0o644),
	})
	if err != nil {
		t.Fatalf("append journal: %v", err)
	}
	if seq <= 0 {
		t.Fatalf("expected positive seq, got %d", seq)
	}

	// Inode does not exist yet.
	if _, err := srv.Metadata.Getattr(ctx, inodeID); err == nil {
		t.Fatal("inode should not exist before replay")
	}

	// Replay applies the pending entry and creates the inode.
	if err := srv.Journal.Replay(ctx, srv.Metadata, v.ID); err != nil {
		t.Fatalf("replay: %v", err)
	}

	// Inode should now exist.
	got, err := srv.Metadata.Lookup(ctx, v.ID, root.ID, "replay.txt")
	if err != nil {
		t.Fatalf("lookup after replay: %v", err)
	}
	if got.Name != "replay.txt" {
		t.Fatalf("unexpected name after replay: %s", got.Name)
	}
}

// ---- Phase 2: lease manager -------------------------------------------------

func TestLeaseAcquireAndRelease(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	l, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c1",
		LeaseType: "write", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	})
	if err != nil {
		t.Fatalf("acquire lease: %v", err)
	}
	if l.ID == "" {
		t.Fatal("no lease ID")
	}

	if err := srv.Leases.Release(ctx, l.ID, "c1"); err != nil {
		t.Fatalf("release lease: %v", err)
	}
}

func TestLeaseConflictWriteWrite(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	_, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c1",
		LeaseType: "write", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	_, err = srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c2",
		LeaseType: "write", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	})
	if err == nil {
		t.Fatal("expected lease conflict for concurrent write leases from different clients")
	}
}

func TestLeaseReadReadAllowed(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	if _, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c1",
		LeaseType: "read", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	}); err != nil {
		t.Fatalf("first read lease: %v", err)
	}
	if _, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c2",
		LeaseType: "read", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	}); err != nil {
		t.Fatalf("second read lease should succeed: %v", err)
	}
}

func TestLeaseExclusiveBlocksAll(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	if _, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c1",
		LeaseType: "exclusive", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	}); err != nil {
		t.Fatalf("exclusive lease: %v", err)
	}

	// Both read and write from another client should be rejected.
	for _, lt := range []string{"read", "write"} {
		_, err := srv.Leases.Acquire(ctx, LeaseRequest{
			VolumeID: v.ID, InodeID: f.ID, ClientID: "c2",
			LeaseType: lt, RangeStart: 0, RangeEnd: -1, Epoch: 1,
		})
		if err == nil {
			t.Fatalf("expected conflict for %s lease against exclusive", lt)
		}
	}
}

func TestLeaseSameClientReacquire(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	v, _ := srv.Volumes.Create(ctx, CreateVolumeOpts{Name: "vol", SizeBytes: 1024})
	root, _ := srv.Metadata.EnsureRoot(ctx, v.ID)
	f, _ := srv.Metadata.Create(ctx, CreateReq{VolumeID: v.ID, ParentID: root.ID, Name: "f.txt", ClientID: "c1"})

	// Same client should be allowed to re-acquire without conflict.
	if _, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c1",
		LeaseType: "write", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	}); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := srv.Leases.Acquire(ctx, LeaseRequest{
		VolumeID: v.ID, InodeID: f.ID, ClientID: "c1",
		LeaseType: "write", RangeStart: 0, RangeEnd: -1, Epoch: 1,
	}); err != nil {
		t.Fatalf("re-acquire by same client: %v", err)
	}
}
