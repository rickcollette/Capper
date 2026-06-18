package registry_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/registry"
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

func openStore(t *testing.T) *registry.Store {
	t.Helper()
	db := openDB(t)
	if err := registry.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return registry.NewStore(db)
}

func openManager(t *testing.T) *registry.Manager {
	t.Helper()
	s := openStore(t)
	root := t.TempDir()
	mgr := registry.NewManager(s, root)
	if err := mgr.EnsureRoot(); err != nil {
		t.Fatalf("EnsureRoot: %v", err)
	}
	return mgr
}

// ---- schema -----------------------------------------------------------------

func TestInitSchemaIdempotent(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 3; i++ {
		if err := registry.InitSchema(db); err != nil {
			t.Fatalf("InitSchema pass %d: %v", i, err)
		}
	}
}

// ---- ParseRef ---------------------------------------------------------------

func TestParseRef(t *testing.T) {
	cases := []struct {
		in       string
		wantReg  string
		wantName string
		wantVer  string
	}{
		{"local/web:0.1.0", "local", "web", "0.1.0"},
		{"local/web", "local", "web", "latest"},
		{"web:0.1.0", "", "web", "0.1.0"},
		{"web", "", "web", "latest"},
		{"local/ns/image:v2", "local", "ns/image", "v2"},
	}
	for _, tc := range cases {
		r := registry.ParseRef(tc.in)
		if r.Registry != tc.wantReg || r.Name != tc.wantName || r.Version != tc.wantVer {
			t.Errorf("ParseRef(%q) = {%q,%q,%q}, want {%q,%q,%q}",
				tc.in, r.Registry, r.Name, r.Version,
				tc.wantReg, tc.wantName, tc.wantVer)
		}
	}
}

// ---- registry store ---------------------------------------------------------

func TestRegistryInsertAndGet(t *testing.T) {
	s := openStore(t)
	r := registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"}
	if err := s.InsertRegistry(r); err != nil {
		t.Fatalf("InsertRegistry: %v", err)
	}
	got, err := s.GetRegistry("local")
	if err != nil {
		t.Fatalf("GetRegistry: %v", err)
	}
	if got.Backend != registry.BackendFilesystem {
		t.Fatalf("backend mismatch: %s", got.Backend)
	}
}

func TestRegistryDeleteCascades(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	_ = s.UpsertImage(registry.RegistryImage{ID: "i1", RegistryID: "r1", Name: "web", Version: "0.1.0", Path: "/tmp/web.cap", CreatedAt: "t"})
	_ = s.UpsertArtifact(registry.Artifact{ID: "a1", RegistryID: "r1", Name: "bundle", Version: "0.1.0", Path: "/tmp/bundle", CreatedAt: "t"})
	if err := s.DeleteRegistry("r1"); err != nil {
		t.Fatalf("DeleteRegistry: %v", err)
	}
	imgs, _ := s.ListImages("r1")
	if len(imgs) != 0 {
		t.Fatalf("expected images cascade deleted, got %d", len(imgs))
	}
	arts, _ := s.ListArtifacts("r1")
	if len(arts) != 0 {
		t.Fatalf("expected artifacts cascade deleted, got %d", len(arts))
	}
}

// ---- image store ------------------------------------------------------------

func TestImageUpsertAndGet(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	img := registry.RegistryImage{
		ID: "i1", RegistryID: "r1", Name: "web", Version: "0.1.0",
		Digest: "sha256abc", Path: "/tmp/web.cap", Signed: true, CreatedAt: "t",
	}
	if err := s.UpsertImage(img); err != nil {
		t.Fatalf("UpsertImage: %v", err)
	}
	got, err := s.GetImage("r1", "web", "0.1.0")
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if !got.Signed {
		t.Fatal("signed flag not stored")
	}
	if got.RegistryName != "local" {
		t.Fatalf("registry name join failed: %q", got.RegistryName)
	}
}

func TestImageUpsertOverwrites(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	_ = s.UpsertImage(registry.RegistryImage{ID: "i1", RegistryID: "r1", Name: "web", Version: "0.1.0", Digest: "old", Path: "/tmp/old.cap", CreatedAt: "t"})
	_ = s.UpsertImage(registry.RegistryImage{ID: "i2", RegistryID: "r1", Name: "web", Version: "0.1.0", Digest: "new", Path: "/tmp/new.cap", CreatedAt: "t2"})
	got, _ := s.GetImage("r1", "web", "0.1.0")
	if got.Digest != "new" {
		t.Fatalf("expected digest 'new', got %q", got.Digest)
	}
}

func TestImageList(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	for _, ver := range []string{"0.1.0", "0.2.0", "latest"} {
		_ = s.UpsertImage(registry.RegistryImage{ID: ver, RegistryID: "r1", Name: "web", Version: ver, Path: "/tmp/web.cap", CreatedAt: "t"})
	}
	imgs, err := s.ListImages("r1")
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(imgs) != 3 {
		t.Fatalf("expected 3, got %d", len(imgs))
	}
}

func TestImageDelete(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	_ = s.UpsertImage(registry.RegistryImage{ID: "i1", RegistryID: "r1", Name: "web", Version: "0.1.0", Path: "/tmp/web.cap", CreatedAt: "t"})
	if err := s.DeleteImage("r1", "web", "0.1.0"); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if _, err := s.GetImage("r1", "web", "0.1.0"); err == nil {
		t.Fatal("expected error after delete")
	}
}

// ---- artifact store ---------------------------------------------------------

func TestArtifactUpsertAndGet(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	a := registry.Artifact{
		ID: "a1", RegistryID: "r1", Name: "app-bundle", Version: "0.1.0",
		Type: "tar.zst", Digest: "sha256abc", Path: "/tmp/bundle.tar.zst",
		SizeBytes: 1000, Labels: map[string]string{"project": "demo"}, CreatedAt: "t",
	}
	if err := s.UpsertArtifact(a); err != nil {
		t.Fatalf("UpsertArtifact: %v", err)
	}
	got, err := s.GetArtifact("r1", "app-bundle", "0.1.0")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if got.Labels["project"] != "demo" {
		t.Fatalf("labels not stored: %v", got.Labels)
	}
	if got.RegistryName != "local" {
		t.Fatalf("registry name join failed: %q", got.RegistryName)
	}
}

func TestArtifactList(t *testing.T) {
	s := openStore(t)
	_ = s.InsertRegistry(registry.Registry{ID: "r1", Name: "local", Backend: registry.BackendFilesystem, Path: "/tmp/local", CreatedAt: "t"})
	for _, n := range []string{"a", "b", "c"} {
		_ = s.UpsertArtifact(registry.Artifact{ID: n, RegistryID: "r1", Name: n, Version: "1.0", Path: "/tmp/" + n, CreatedAt: "t"})
	}
	arts, err := s.ListArtifacts("r1")
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(arts) != 3 {
		t.Fatalf("expected 3, got %d", len(arts))
	}
}

// ---- manager ----------------------------------------------------------------

func TestManagerInitIdempotent(t *testing.T) {
	mgr := openManager(t)
	r1, err := mgr.Init("local")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	r2, err := mgr.Init("local")
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if r1.ID != r2.ID {
		t.Fatalf("idempotent Init should return same registry, got %s vs %s", r1.ID, r2.ID)
	}
	// Directories exist.
	if _, err := os.Stat(filepath.Join(r1.Path, "images")); err != nil {
		t.Fatalf("images dir not created: %v", err)
	}
}

func TestManagerDeleteRegistry(t *testing.T) {
	mgr := openManager(t)
	r, _ := mgr.Init("local")
	if err := mgr.DeleteRegistry("local"); err != nil {
		t.Fatalf("DeleteRegistry: %v", err)
	}
	if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
		t.Fatal("registry dir should be removed")
	}
}

func TestManagerPushAndPull(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.Init("local")

	// Create a fake .cap file.
	src := filepath.Join(t.TempDir(), "web.cap")
	if err := os.WriteFile(src, []byte("fake cap content"), 0o644); err != nil {
		t.Fatal(err)
	}

	img, err := mgr.Push("local", "web", "0.1.0", src)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if img.Digest == "" {
		t.Fatal("digest should be set after push")
	}
	if img.RegistryName != "local" {
		t.Fatalf("registry name not set: %q", img.RegistryName)
	}
	if _, err := os.Stat(img.Path); err != nil {
		t.Fatalf("pushed file not found: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "web-pulled.cap")
	pulled, err := mgr.Pull("local", "web", "0.1.0", dest)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if pulled.Digest != img.Digest {
		t.Fatalf("digest mismatch after pull: %s vs %s", pulled.Digest, img.Digest)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "fake cap content" {
		t.Fatalf("pulled content mismatch: %q", data)
	}
}

func TestManagerTagImage(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.Init("local")

	src := filepath.Join(t.TempDir(), "web.cap")
	_ = os.WriteFile(src, []byte("content"), 0o644)
	_, _ = mgr.Push("local", "web", "0.1.0", src)

	tagged, err := mgr.TagImage("local", "web", "0.1.0", "latest")
	if err != nil {
		t.Fatalf("TagImage: %v", err)
	}
	if tagged.Version != "latest" {
		t.Fatalf("expected version 'latest', got %s", tagged.Version)
	}

	imgs, _ := mgr.ListImages("local")
	if len(imgs) != 2 {
		t.Fatalf("expected 2 versions (0.1.0 + latest), got %d", len(imgs))
	}
}

func TestManagerDeleteImage(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.Init("local")
	src := filepath.Join(t.TempDir(), "web.cap")
	_ = os.WriteFile(src, []byte("content"), 0o644)
	img, _ := mgr.Push("local", "web", "0.1.0", src)

	if err := mgr.DeleteImage("local", "web", "0.1.0"); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if _, err := os.Stat(img.Path); !os.IsNotExist(err) {
		t.Fatal("image file should be removed after delete")
	}
}

func TestManagerPutAndGetArtifact(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.Init("local")

	src := filepath.Join(t.TempDir(), "app.tar.zst")
	if err := os.WriteFile(src, []byte("artifact bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	labels := map[string]string{"project": "demo"}
	a, err := mgr.PutArtifact("local", "app-bundle", "0.1.0", "", src, labels)
	if err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}
	if a.Type != "tar.zst" {
		t.Fatalf("type inference failed: %q", a.Type)
	}
	if a.Labels["project"] != "demo" {
		t.Fatalf("labels not stored: %v", a.Labels)
	}

	dest := filepath.Join(t.TempDir(), "got.tar.zst")
	got, err := mgr.GetArtifact("local", "app-bundle", "0.1.0", dest)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if got.Digest != a.Digest {
		t.Fatalf("digest mismatch: %s vs %s", got.Digest, a.Digest)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "artifact bytes" {
		t.Fatalf("artifact content mismatch: %q", data)
	}
}

func TestManagerDeleteArtifact(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.Init("local")
	src := filepath.Join(t.TempDir(), "bundle.tar.zst")
	_ = os.WriteFile(src, []byte("data"), 0o644)
	a, _ := mgr.PutArtifact("local", "bundle", "1.0", "", src, nil)

	if err := mgr.DeleteArtifact("local", "bundle", "1.0"); err != nil {
		t.Fatalf("DeleteArtifact: %v", err)
	}
	if _, err := os.Stat(a.Path); !os.IsNotExist(err) {
		t.Fatal("artifact file should be removed after delete")
	}
}

func TestManagerGC(t *testing.T) {
	mgr := openManager(t)
	r, _ := mgr.Init("local")

	// Put a known artifact.
	src := filepath.Join(t.TempDir(), "known.tar.zst")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	_, _ = mgr.PutArtifact("local", "known", "1.0", "", src, nil)

	// Write an orphan file directly into the artifacts dir.
	orphan := filepath.Join(r.Path, "artifacts", "orphan")
	_ = os.MkdirAll(orphan, 0o700)

	n, err := mgr.GC("local")
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 orphan removed, got %d", n)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphan dir should be removed by GC")
	}
}

func TestManagerListAllImages(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.Init("reg-a")
	_, _ = mgr.Init("reg-b")

	for _, reg := range []string{"reg-a", "reg-b"} {
		src := filepath.Join(t.TempDir(), "img.cap")
		_ = os.WriteFile(src, []byte("x"), 0o644)
		_, _ = mgr.Push(reg, "web", "1.0", src)
	}

	// List all (no filter).
	all, err := mgr.ListImages("")
	if err != nil {
		t.Fatalf("ListImages all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
}
