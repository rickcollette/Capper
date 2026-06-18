package storage_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/storage"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func openStore(t *testing.T) *storage.Store {
	t.Helper()
	db := openDB(t)
	if err := storage.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return storage.NewStore(db)
}

func openManager(t *testing.T) (*storage.Manager, storage.Paths) {
	t.Helper()
	s := openStore(t)
	tmp := t.TempDir()
	paths := storage.Paths{
		Volumes:   filepath.Join(tmp, "volumes"),
		Buckets:   filepath.Join(tmp, "objects"),
		Snapshots: filepath.Join(tmp, "snapshots"),
	}
	mgr := storage.NewManager(s, paths)
	if err := mgr.EnsurePaths(); err != nil {
		t.Fatalf("EnsurePaths: %v", err)
	}
	return mgr, paths
}

// ---- schema -----------------------------------------------------------------

func TestInitSchemaIdempotent(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 3; i++ {
		if err := storage.InitSchema(db); err != nil {
			t.Fatalf("InitSchema pass %d: %v", i, err)
		}
	}
}

// ---- volume store -----------------------------------------------------------

func TestVolumeInsertAndGet(t *testing.T) {
	s := openStore(t)
	v := storage.Volume{
		ID:        "v1",
		Name:      "pgdata",
		SizeBytes: 10 * 1 << 30,
		Class:     storage.VolumeClassLocalSSD,
		Backend:   storage.VolumeBackendDirectory,
		Path:      "/tmp/pgdata",
		CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.InsertVolume(v); err != nil {
		t.Fatalf("InsertVolume: %v", err)
	}
	got, err := s.GetVolume("pgdata")
	if err != nil {
		t.Fatalf("GetVolume by name: %v", err)
	}
	if got.SizeBytes != v.SizeBytes {
		t.Fatalf("size mismatch: %d", got.SizeBytes)
	}
}

func TestVolumeAttachDetach(t *testing.T) {
	s := openStore(t)
	_ = s.InsertVolume(storage.Volume{ID: "v1", Name: "data", Backend: storage.VolumeBackendDirectory, Path: "/tmp/data", CreatedAt: "t"})
	if err := s.UpdateVolumeAttachment("data", "inst_abc", "/var/lib/data"); err != nil {
		t.Fatalf("UpdateVolumeAttachment: %v", err)
	}
	v, _ := s.GetVolume("data")
	if v.AttachedInstanceID != "inst_abc" {
		t.Fatalf("expected attachment, got %q", v.AttachedInstanceID)
	}
	_ = s.UpdateVolumeAttachment("data", "", "")
	v, _ = s.GetVolume("data")
	if v.AttachedInstanceID != "" {
		t.Fatalf("expected detached, got %q", v.AttachedInstanceID)
	}
}

func TestVolumeList(t *testing.T) {
	s := openStore(t)
	for _, n := range []string{"c", "a", "b"} {
		_ = s.InsertVolume(storage.Volume{ID: n, Name: n, Backend: storage.VolumeBackendDirectory, Path: "/tmp/" + n, CreatedAt: "t"})
	}
	vs, err := s.ListVolumes()
	if err != nil {
		t.Fatalf("ListVolumes: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("expected 3, got %d", len(vs))
	}
	if vs[0].Name != "a" {
		t.Fatalf("expected sorted, got %s first", vs[0].Name)
	}
}

func TestVolumeDelete(t *testing.T) {
	s := openStore(t)
	_ = s.InsertVolume(storage.Volume{ID: "v1", Name: "old", Backend: storage.VolumeBackendDirectory, Path: "/tmp/old", CreatedAt: "t"})
	if err := s.DeleteVolume("v1"); err != nil {
		t.Fatalf("DeleteVolume: %v", err)
	}
	if _, err := s.GetVolume("old"); err == nil {
		t.Fatal("expected error after delete")
	}
}

// ---- bucket store -----------------------------------------------------------

func TestBucketInsertAndGet(t *testing.T) {
	s := openStore(t)
	b := storage.Bucket{
		ID: "b1", Name: "artifacts", Backend: storage.BucketBackendLocal,
		Path: "/tmp/artifacts", Versioning: true, CreatedAt: "t",
	}
	if err := s.InsertBucket(b); err != nil {
		t.Fatalf("InsertBucket: %v", err)
	}
	got, err := s.GetBucket("artifacts")
	if err != nil {
		t.Fatalf("GetBucket: %v", err)
	}
	if !got.Versioning {
		t.Fatal("versioning not stored")
	}
}

func TestBucketDeleteCascadesObjects(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt"})

	src := filepath.Join(t.TempDir(), "file.txt")
	_ = os.WriteFile(src, []byte("data"), 0o644)
	_, _ = mgr.PutObject("bkt", "file.txt", src)

	if err := mgr.DeleteBucket("bkt", true); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	// Bucket no longer exists — ListObjects should error, not return objects.
	objs, err := mgr.ListObjects("bkt", "")
	if err == nil && len(objs) != 0 {
		t.Fatalf("expected no objects after bucket delete, got %d", len(objs))
	}
}

// ---- object operations (via Manager / filesystem) ---------------------------

func TestObjectPutAndList(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt"})

	tmp := t.TempDir()
	for _, key := range []string{"a/1.txt", "a/2.txt", "b/3.txt"} {
		f := filepath.Join(tmp, filepath.Base(key))
		_ = os.WriteFile(f, []byte("x"), 0o644)
		if _, err := mgr.PutObject("bkt", key, f); err != nil {
			t.Fatalf("PutObject %s: %v", key, err)
		}
	}
	all, err := mgr.ListObjects("bkt", "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	filtered, _ := mgr.ListObjects("bkt", "a/")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 with prefix a/, got %d", len(filtered))
	}
}

func TestObjectPutOverwrites(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt"})

	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "v1.txt")
	f2 := filepath.Join(tmp, "v2.txt")
	_ = os.WriteFile(f1, []byte("short"), 0o644)
	_ = os.WriteFile(f2, []byte("longer content"), 0o644)

	_, _ = mgr.PutObject("bkt", "file", f1)
	o, err := mgr.PutObject("bkt", "file", f2)
	if err != nil {
		t.Fatalf("PutObject overwrite: %v", err)
	}
	if o.SizeBytes != int64(len("longer content")) {
		t.Fatalf("expected size %d after overwrite, got %d", len("longer content"), o.SizeBytes)
	}
}

func TestBucketQuotaOverwriteUsesFinalSize(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt", QuotaBytes: 10})

	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "v1.txt")
	f2 := filepath.Join(tmp, "v2.txt")
	_ = os.WriteFile(f1, []byte("12345678"), 0o644)
	_ = os.WriteFile(f2, []byte("123456789"), 0o644)

	if _, err := mgr.PutObject("bkt", "file", f1); err != nil {
		t.Fatalf("initial PutObject: %v", err)
	}
	if _, err := mgr.PutObject("bkt", "file", f2); err != nil {
		t.Fatalf("overwrite should use final bucket size, got: %v", err)
	}
	if used := mgr.BucketUsageBytes("bkt"); used != 9 {
		t.Fatalf("expected bucket usage 9 after overwrite, got %d", used)
	}
}

func TestBucketQuotaRejectsOverwriteBeyondFinalSize(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt", QuotaBytes: 10})

	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "v1.txt")
	f2 := filepath.Join(tmp, "v2.txt")
	_ = os.WriteFile(f1, []byte("12345"), 0o644)
	_ = os.WriteFile(f2, []byte("12345678901"), 0o644)

	if _, err := mgr.PutObject("bkt", "file", f1); err != nil {
		t.Fatalf("initial PutObject: %v", err)
	}
	if _, err := mgr.PutObject("bkt", "file", f2); err == nil {
		t.Fatal("expected overwrite beyond final quota to fail")
	}
	if used := mgr.BucketUsageBytes("bkt"); used != 5 {
		t.Fatalf("failed overwrite should leave old object usage, got %d", used)
	}
}

func TestObjectDelete(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt"})

	src := filepath.Join(t.TempDir(), "file.txt")
	_ = os.WriteFile(src, []byte("hello"), 0o644)
	_, _ = mgr.PutObject("bkt", "file.txt", src)

	if err := mgr.DeleteObject("bkt", "file.txt"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "out.txt")
	if err := mgr.GetObject("bkt", "file.txt", dest); err == nil {
		t.Fatal("expected error after delete")
	}
}

// ---- snapshot store ---------------------------------------------------------

func TestSnapshotInsertAndGet(t *testing.T) {
	s := openStore(t)
	snap := storage.Snapshot{
		ID: "sn1", Name: "pgdata-pre-upgrade",
		SourceType: storage.SnapshotSourceVolume, SourceID: "v1",
		Path:   "/tmp/snapshots/pgdata-pre-upgrade.tar.zst",
		Digest: "abcdef", SizeBytes: 1000, CreatedAt: "t",
	}
	if err := s.InsertSnapshot(snap); err != nil {
		t.Fatalf("InsertSnapshot: %v", err)
	}
	got, err := s.GetSnapshot("pgdata-pre-upgrade")
	if err != nil {
		t.Fatalf("GetSnapshot by name: %v", err)
	}
	if got.SourceID != "v1" {
		t.Fatalf("source ID mismatch: %s", got.SourceID)
	}
}

func TestSnapshotList(t *testing.T) {
	s := openStore(t)
	for _, n := range []string{"snap1", "snap2", "snap3"} {
		_ = s.InsertSnapshot(storage.Snapshot{ID: n, Name: n, SourceType: storage.SnapshotSourceVolume, SourceID: "v1", Path: "/tmp/" + n, CreatedAt: "t"})
	}
	all, _ := s.ListSnapshots("")
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	bySource, _ := s.ListSnapshots("v1")
	if len(bySource) != 3 {
		t.Fatalf("expected 3 for source v1, got %d", len(bySource))
	}
}

func TestSnapshotDelete(t *testing.T) {
	s := openStore(t)
	_ = s.InsertSnapshot(storage.Snapshot{ID: "sn1", Name: "old", SourceType: storage.SnapshotSourceVolume, SourceID: "v1", Path: "/tmp/old", CreatedAt: "t"})
	if err := s.DeleteSnapshot("sn1"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
	if _, err := s.GetSnapshot("old"); err == nil {
		t.Fatal("expected error after delete")
	}
}

// ---- manager ----------------------------------------------------------------

func TestManagerCreateAndDeleteVolume(t *testing.T) {
	mgr, paths := openManager(t)
	v, err := mgr.CreateVolume(storage.CreateVolumeOptions{Name: "pgdata", SizeBytes: 10 << 30, Class: storage.VolumeClassLocal})
	if err != nil {
		t.Fatalf("CreateVolume: %v", err)
	}
	if v.Path != filepath.Join(paths.Volumes, "pgdata") {
		t.Fatalf("unexpected path: %s", v.Path)
	}
	if _, err := os.Stat(v.Path); err != nil {
		t.Fatalf("volume dir not created: %v", err)
	}
	if err := mgr.DeleteVolume("pgdata"); err != nil {
		t.Fatalf("DeleteVolume: %v", err)
	}
	if _, err := os.Stat(v.Path); !os.IsNotExist(err) {
		t.Fatal("volume dir should be removed after delete")
	}
}

func TestManagerAttachDetachVolume(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateVolume(storage.CreateVolumeOptions{Name: "data"})
	if err := mgr.AttachVolume("data", "inst_abc", "/data"); err != nil {
		t.Fatalf("AttachVolume: %v", err)
	}
	v, _ := mgr.GetVolume("data")
	if v.AttachedInstanceID != "inst_abc" {
		t.Fatalf("attachment not recorded: %+v", v)
	}
	// Double-attach should fail.
	if err := mgr.AttachVolume("data", "inst_xyz", "/data"); err == nil {
		t.Fatal("expected error on double-attach")
	}
	if err := mgr.DetachVolume("data"); err != nil {
		t.Fatalf("DetachVolume: %v", err)
	}
	v, _ = mgr.GetVolume("data")
	if v.AttachedInstanceID != "" {
		t.Fatalf("expected detached, got %q", v.AttachedInstanceID)
	}
}

func TestManagerDeleteAttachedVolumeBlocked(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateVolume(storage.CreateVolumeOptions{Name: "data"})
	_ = mgr.AttachVolume("data", "inst_abc", "/data")
	if err := mgr.DeleteVolume("data"); err == nil {
		t.Fatal("expected error deleting attached volume")
	}
}

func TestManagerCreateAndDeleteBucket(t *testing.T) {
	mgr, paths := openManager(t)
	b, err := mgr.CreateBucket(storage.CreateBucketOptions{Name: "artifacts"})
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if b.Path != filepath.Join(paths.Buckets, "artifacts") {
		t.Fatalf("unexpected path: %s", b.Path)
	}
	if _, err := os.Stat(b.Path); err != nil {
		t.Fatalf("bucket dir not created: %v", err)
	}
	if err := mgr.DeleteBucket("artifacts", false); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
}

func TestManagerDeleteNonEmptyBucketBlocked(t *testing.T) {
	mgr, _ := openManager(t)
	b, _ := mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt"})

	// Put an actual file.
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = mgr.PutObject(b.Name, "test.txt", tmpFile)

	if err := mgr.DeleteBucket("bkt", false); err == nil {
		t.Fatal("expected error when bucket is not empty")
	}
	if err := mgr.DeleteBucket("bkt", true); err != nil {
		t.Fatalf("force delete should succeed: %v", err)
	}
}

func TestManagerPutGetObject(t *testing.T) {
	mgr, _ := openManager(t)
	_, _ = mgr.CreateBucket(storage.CreateBucketOptions{Name: "bkt"})

	// Create a source file.
	src := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	o, err := mgr.PutObject("bkt", "hello.txt", src)
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if o.SizeBytes != 11 {
		t.Fatalf("expected size 11, got %d", o.SizeBytes)
	}
	if o.Digest == "" {
		t.Fatal("digest should be set")
	}

	dest := filepath.Join(t.TempDir(), "out.txt")
	if err := mgr.GetObject("bkt", "hello.txt", dest); err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "hello world" {
		t.Fatalf("content mismatch: %q", data)
	}
}

func TestManagerSnapshotAndRestoreVolume(t *testing.T) {
	mgr, _ := openManager(t)
	v, _ := mgr.CreateVolume(storage.CreateVolumeOptions{Name: "pgdata"})

	// Write some data into the volume.
	if err := os.WriteFile(filepath.Join(v.Path, "pg.conf"), []byte("max_connections=100"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap, err := mgr.SnapshotVolume("pgdata", "pgdata-backup")
	if err != nil {
		t.Fatalf("SnapshotVolume: %v", err)
	}
	if snap.SizeBytes == 0 {
		t.Fatal("snapshot size should be > 0")
	}
	if snap.Digest == "" {
		t.Fatal("snapshot digest should be set")
	}
	if _, err := os.Stat(snap.Path); err != nil {
		t.Fatalf("snapshot file not created: %v", err)
	}

	// Corrupt the volume.
	if err := os.Remove(filepath.Join(v.Path, "pg.conf")); err != nil {
		t.Fatal(err)
	}

	// Restore from snapshot.
	if err := mgr.RestoreSnapshot("pgdata-backup", "pgdata"); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}

	restored, _ := os.ReadFile(filepath.Join(v.Path, "pg.conf"))
	if string(restored) != "max_connections=100" {
		t.Fatalf("restored content mismatch: %q", restored)
	}
}

func TestManagerDeleteSnapshot(t *testing.T) {
	mgr, _ := openManager(t)
	v, _ := mgr.CreateVolume(storage.CreateVolumeOptions{Name: "data"})
	_ = os.WriteFile(filepath.Join(v.Path, "file.txt"), []byte("x"), 0o644)

	snap, _ := mgr.SnapshotVolume("data", "backup")
	if err := mgr.DeleteSnapshot("backup"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
	if _, err := os.Stat(snap.Path); !os.IsNotExist(err) {
		t.Fatal("snapshot file should be removed after delete")
	}
}
