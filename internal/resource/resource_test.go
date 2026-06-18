package resource_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/resource"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := resource.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestNewIDFormat verifies the prefix_16hex format and uniqueness.
func TestNewIDFormat(t *testing.T) {
	id := resource.NewID("net")
	parts := strings.SplitN(id, "_", 2)
	if len(parts) != 2 {
		t.Fatalf("expected prefix_hex, got %q", id)
	}
	if parts[0] != "net" {
		t.Errorf("expected prefix 'net', got %q", parts[0])
	}
	if len(parts[1]) != 16 {
		t.Errorf("expected 16 hex chars, got %d in %q", len(parts[1]), parts[1])
	}
	// basic uniqueness: two IDs must differ
	if resource.NewID("x") == resource.NewID("x") {
		t.Error("two consecutive NewID calls returned equal values")
	}
}

// TestMatchLabels covers the selector semantics.
func TestMatchLabels(t *testing.T) {
	labels := map[string]string{"app": "web", "env": "prod"}

	if !resource.MatchLabels(map[string]string{"app": "web"}, labels) {
		t.Error("subset selector should match")
	}
	if !resource.MatchLabels(nil, labels) {
		t.Error("empty selector should match everything")
	}
	if resource.MatchLabels(map[string]string{"app": "db"}, labels) {
		t.Error("wrong value should not match")
	}
	if resource.MatchLabels(map[string]string{"missing": "key"}, labels) {
		t.Error("absent key should not match")
	}
}

// TestStoreRegisterAndGet verifies the round-trip through Register and Get.
func TestStoreRegisterAndGet(t *testing.T) {
	db := openTestDB(t)
	s := resource.NewStore(db)

	r := resource.Resource{
		ID:      resource.NewID("net"),
		Type:    resource.TypeNetwork,
		Name:    "devnet",
		Project: "default",
		Owner:   "user:rick",
		Labels:  map[string]string{"env": "dev"},
		Status:  resource.StatusActive,
	}
	if err := s.Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := s.Get(r.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != r.Name {
		t.Errorf("Name: want %q got %q", r.Name, got.Name)
	}
	if got.Labels["env"] != "dev" {
		t.Errorf("Labels not preserved: %v", got.Labels)
	}
	if got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Error("timestamps should be set")
	}
}

// TestStoreUpdateStatus verifies that UpdateStatus changes the status field.
func TestStoreUpdateStatus(t *testing.T) {
	db := openTestDB(t)
	s := resource.NewStore(db)

	r := resource.Resource{
		ID:     resource.NewID("vol"),
		Type:   resource.TypeVolume,
		Name:   "myvol",
		Status: resource.StatusCreating,
	}
	if err := s.Register(r); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateStatus(r.ID, resource.StatusActive); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ := s.Get(r.ID)
	if got.Status != resource.StatusActive {
		t.Errorf("expected active, got %q", got.Status)
	}
}

// TestStoreList verifies filtering by type and project.
func TestStoreList(t *testing.T) {
	db := openTestDB(t)
	s := resource.NewStore(db)

	for i, spec := range []struct {
		typ, proj string
	}{
		{resource.TypeNetwork, "alpha"},
		{resource.TypeNetwork, "beta"},
		{resource.TypeVolume, "alpha"},
	} {
		_ = s.Register(resource.Resource{
			ID: resource.NewID("r"), Type: spec.typ,
			Name: "r" + string(rune('0'+i)), Project: spec.proj,
			Status: resource.StatusActive,
		})
	}

	nets, err := s.List(resource.TypeNetwork, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 {
		t.Errorf("expected 1 network in alpha, got %d", len(nets))
	}

	all, err := s.List(resource.TypeNetwork, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 networks total, got %d", len(all))
	}
}

// TestStoreListBySelector verifies label-selector filtering.
func TestStoreListBySelector(t *testing.T) {
	db := openTestDB(t)
	s := resource.NewStore(db)

	for _, spec := range []struct {
		name   string
		labels map[string]string
	}{
		{"web1", map[string]string{"app": "web", "env": "prod"}},
		{"web2", map[string]string{"app": "web", "env": "staging"}},
		{"db1", map[string]string{"app": "db", "env": "prod"}},
	} {
		_ = s.Register(resource.Resource{
			ID: resource.NewID("i"), Type: resource.TypeInstance,
			Name: spec.name, Project: "default", Labels: spec.labels,
			Status: resource.StatusActive,
		})
	}

	webProd, _ := s.ListBySelector(resource.TypeInstance, "default",
		map[string]string{"app": "web", "env": "prod"})
	if len(webProd) != 1 || webProd[0].Name != "web1" {
		t.Errorf("expected web1 only, got %v", webProd)
	}

	allWeb, _ := s.ListBySelector(resource.TypeInstance, "default",
		map[string]string{"app": "web"})
	if len(allWeb) != 2 {
		t.Errorf("expected 2 web instances, got %d", len(allWeb))
	}
}

// TestStoreDelete verifies that a registered resource can be deleted.
func TestStoreDelete(t *testing.T) {
	db := openTestDB(t)
	s := resource.NewStore(db)

	r := resource.Resource{
		ID: resource.NewID("img"), Type: resource.TypeImage,
		Name: "hello.cap", Status: resource.StatusActive,
	}
	if err := s.Register(r); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(r.ID); err == nil {
		t.Error("second Delete should return an error")
	}
}

// TestStoreGetByName verifies lookup by type+name+project.
func TestStoreGetByName(t *testing.T) {
	db := openTestDB(t)
	s := resource.NewStore(db)

	r := resource.Resource{
		ID: resource.NewID("net"), Type: resource.TypeNetwork,
		Name: "prodnet", Project: "infra", Status: resource.StatusActive,
	}
	if err := s.Register(r); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByName(resource.TypeNetwork, "prodnet", "infra")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("ID mismatch: want %q got %q", r.ID, got.ID)
	}

	_, err = s.GetByName(resource.TypeNetwork, "prodnet", "other-project")
	if err == nil {
		t.Error("expected error for wrong project, got nil")
	}
}

// TestInitSchemaIdempotent verifies that calling InitSchema twice does not error.
func TestInitSchemaIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := resource.InitSchema(db); err != nil {
		t.Errorf("second InitSchema call: %v", err)
	}
}
