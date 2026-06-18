package compute_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/compute"
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

func openStore(t *testing.T) *compute.Store {
	t.Helper()
	db := openDB(t)
	if err := compute.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return compute.NewStore(db)
}

// ---- schema -----------------------------------------------------------------

func TestInitSchemaIdempotent(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 3; i++ {
		if err := compute.InitSchema(db); err != nil {
			t.Fatalf("InitSchema pass %d: %v", i, err)
		}
	}
}

// ---- host store -------------------------------------------------------------

func TestHostUpsertAndGet(t *testing.T) {
	s := openStore(t)
	h := compute.Host{
		ID:        "h1",
		Name:      "local",
		Status:    compute.HostStatusReady,
		Labels:    map[string]string{"env": "test"},
		CPUCount:  4,
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.UpsertHost(h); err != nil {
		t.Fatalf("UpsertHost: %v", err)
	}
	got, err := s.GetHost("local")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if got.Name != "local" || got.CPUCount != 4 {
		t.Fatalf("unexpected host: %+v", got)
	}
	if got.Labels["env"] != "test" {
		t.Fatalf("labels not preserved: %v", got.Labels)
	}
}

func TestHostUpsertIsIdempotent(t *testing.T) {
	s := openStore(t)
	h := compute.Host{ID: "h1", Name: "local", Status: compute.HostStatusReady, CPUCount: 2, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"}
	_ = s.UpsertHost(h)
	h.CPUCount = 8
	if err := s.UpsertHost(h); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, _ := s.GetHost("local")
	if got.CPUCount != 8 {
		t.Fatalf("upsert should update CPUCount, got %d", got.CPUCount)
	}
}

func TestHostList(t *testing.T) {
	s := openStore(t)
	for _, name := range []string{"b", "a", "c"} {
		_ = s.UpsertHost(compute.Host{ID: name, Name: name, Status: compute.HostStatusReady, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"})
	}
	hosts, err := s.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(hosts) != 3 {
		t.Fatalf("expected 3, got %d", len(hosts))
	}
	// Should be alphabetical.
	if hosts[0].Name != "a" {
		t.Fatalf("expected sorted: got %s first", hosts[0].Name)
	}
}

func TestHostUpdateStatus(t *testing.T) {
	s := openStore(t)
	_ = s.UpsertHost(compute.Host{ID: "h1", Name: "local", Status: compute.HostStatusReady, CreatedAt: "t", UpdatedAt: "t"})
	if err := s.UpdateHostStatus("local", compute.HostStatusDrained, "t2"); err != nil {
		t.Fatalf("UpdateHostStatus: %v", err)
	}
	h, _ := s.GetHost("local")
	if h.Status != compute.HostStatusDrained {
		t.Fatalf("expected drained, got %s", h.Status)
	}
}

func TestProvisionHostReportsRegistrationOnly(t *testing.T) {
	s := openStore(t)
	m := compute.NewManager(s)
	result, err := m.ProvisionHost("10.0.0.10", "root", "/tmp/key", map[string]string{"region": "test"})
	if err != nil {
		t.Fatalf("ProvisionHost: %v", err)
	}
	if result.Status != "registered" {
		t.Fatalf("expected registered status, got %q", result.Status)
	}
	if result.Logs == "" || result.Logs == "host registered (SSH bootstrap stub — run capd install manually)" {
		t.Fatalf("logs should not report stubbed provisioning: %q", result.Logs)
	}
	host, err := s.GetHost(result.HostID)
	if err != nil {
		t.Fatalf("registered host not found: %v", err)
	}
	if host.Labels["region"] != "test" {
		t.Fatalf("labels not preserved: %+v", host.Labels)
	}
}

// ---- template store ---------------------------------------------------------

func makeTemplate(name, image string) compute.Template {
	return compute.Template{
		ID:    "tmpl_" + name,
		Name:  name,
		Image: image,
		Doc: compute.TemplateDoc{
			Name:      name,
			Image:     image,
			Resources: compute.ResourceSpec{MemoryBytes: 536870912},
		},
		CreatedAt: "2024-01-01T00:00:00Z",
	}
}

func TestTemplateInsertAndGet(t *testing.T) {
	s := openStore(t)
	tmpl := makeTemplate("web-small", "web.cap")
	if err := s.InsertTemplate(tmpl); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}
	got, err := s.GetTemplate("web-small")
	if err != nil {
		t.Fatalf("GetTemplate by name: %v", err)
	}
	if got.Image != "web.cap" {
		t.Fatalf("unexpected image: %s", got.Image)
	}
	if got.Doc.Resources.MemoryBytes != 536870912 {
		t.Fatalf("doc.resources not persisted: %+v", got.Doc)
	}
}

func TestTemplateGetByID(t *testing.T) {
	s := openStore(t)
	tmpl := makeTemplate("worker", "worker.cap")
	_ = s.InsertTemplate(tmpl)
	got, err := s.GetTemplate("tmpl_worker")
	if err != nil {
		t.Fatalf("GetTemplate by ID: %v", err)
	}
	if got.Name != "worker" {
		t.Fatalf("name mismatch: %s", got.Name)
	}
}

func TestTemplateList(t *testing.T) {
	s := openStore(t)
	for _, n := range []string{"z", "a", "m"} {
		_ = s.InsertTemplate(makeTemplate(n, n+".cap"))
	}
	ts, err := s.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(ts) != 3 {
		t.Fatalf("expected 3, got %d", len(ts))
	}
	if ts[0].Name != "a" {
		t.Fatalf("expected sorted, got %s first", ts[0].Name)
	}
}

func TestTemplateDelete(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("old", "old.cap"))
	if err := s.DeleteTemplate("tmpl_old"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if _, err := s.GetTemplate("old"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTemplateInUse(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	inUse, _ := s.TemplateInUse("tmpl_web")
	if inUse {
		t.Fatal("should not be in use before any groups")
	}
}

// ---- group store ------------------------------------------------------------

func TestGroupInsertAndGet(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	g := compute.Group{
		ID: "grp_1", Name: "web-asg", TemplateID: "tmpl_web",
		MinSize: 1, DesiredSize: 2, MaxSize: 5,
		Status: compute.GroupStatusActive, CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.InsertGroup(g); err != nil {
		t.Fatalf("InsertGroup: %v", err)
	}
	got, err := s.GetGroup("web-asg")
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.DesiredSize != 2 {
		t.Fatalf("expected desired=2, got %d", got.DesiredSize)
	}
	if got.TemplateName != "web" {
		t.Fatalf("expected template name 'web', got %q", got.TemplateName)
	}
}

func TestGroupUpdateDesired(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	_ = s.InsertGroup(compute.Group{ID: "grp_1", Name: "asg", TemplateID: "tmpl_web", MinSize: 0, DesiredSize: 1, MaxSize: 10, Status: "active", CreatedAt: "t"})
	if err := s.UpdateGroupDesired("asg", 5); err != nil {
		t.Fatalf("UpdateGroupDesired: %v", err)
	}
	g, _ := s.GetGroup("asg")
	if g.DesiredSize != 5 {
		t.Fatalf("expected 5, got %d", g.DesiredSize)
	}
}

func TestTemplateInUseByGroup(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	_ = s.InsertGroup(compute.Group{ID: "grp_1", Name: "asg", TemplateID: "tmpl_web", MinSize: 0, DesiredSize: 1, MaxSize: 5, Status: "active", CreatedAt: "t"})
	inUse, _ := s.TemplateInUse("tmpl_web")
	if !inUse {
		t.Fatal("template should be in use by group")
	}
}

func TestGroupDeleteCleansInstances(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	_ = s.InsertGroup(compute.Group{ID: "grp_1", Name: "asg", TemplateID: "tmpl_web", MinSize: 0, DesiredSize: 1, MaxSize: 5, Status: "active", CreatedAt: "t"})
	_ = s.AddGroupInstance("grp_1", "inst_abc", "t")
	if err := s.DeleteGroup("asg"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, err := s.GetGroup("asg"); err == nil {
		t.Fatal("expected error after delete")
	}
	// Membership records should also be gone.
	ids, _ := s.ListGroupInstances("grp_1")
	if len(ids) != 0 {
		t.Fatalf("expected no group instances after delete, got %v", ids)
	}
}

// ---- group instances --------------------------------------------------------

func TestGroupInstanceTracking(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	_ = s.InsertGroup(compute.Group{ID: "g1", Name: "asg", TemplateID: "tmpl_web", MinSize: 0, DesiredSize: 1, MaxSize: 5, Status: "active", CreatedAt: "t"})

	for _, id := range []string{"i1", "i2", "i3"} {
		if err := s.AddGroupInstance("g1", id, "t"); err != nil {
			t.Fatalf("AddGroupInstance %s: %v", id, err)
		}
	}
	ids, err := s.ListGroupInstances("g1")
	if err != nil {
		t.Fatalf("ListGroupInstances: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3, got %d", len(ids))
	}

	if err := s.RemoveGroupInstance("g1", "i2"); err != nil {
		t.Fatalf("RemoveGroupInstance: %v", err)
	}
	ids, _ = s.ListGroupInstances("g1")
	if len(ids) != 2 {
		t.Fatalf("expected 2 after remove, got %d", len(ids))
	}
}

func TestGroupInstanceAddIdempotent(t *testing.T) {
	s := openStore(t)
	_ = s.InsertTemplate(makeTemplate("web", "web.cap"))
	_ = s.InsertGroup(compute.Group{ID: "g1", Name: "asg", TemplateID: "tmpl_web", MinSize: 0, DesiredSize: 1, MaxSize: 5, Status: "active", CreatedAt: "t"})

	for i := 0; i < 3; i++ {
		if err := s.AddGroupInstance("g1", "i1", "t"); err != nil {
			t.Fatalf("AddGroupInstance attempt %d: %v", i, err)
		}
	}
	ids, _ := s.ListGroupInstances("g1")
	if len(ids) != 1 {
		t.Fatalf("expected 1 (idempotent), got %d", len(ids))
	}
}

// ---- manager ----------------------------------------------------------------

func openManager(t *testing.T) *compute.Manager {
	t.Helper()
	s := openStore(t)
	return compute.NewManager(s)
}

func TestManagerRegisterLocal(t *testing.T) {
	mgr := openManager(t)
	h, err := mgr.RegisterLocal()
	if err != nil {
		t.Fatalf("RegisterLocal: %v", err)
	}
	if h.Name != "local" {
		t.Fatalf("expected name 'local', got %s", h.Name)
	}
	if h.Status != compute.HostStatusReady {
		t.Fatalf("expected ready, got %s", h.Status)
	}
	// Idempotent second call.
	if _, err := mgr.RegisterLocal(); err != nil {
		t.Fatalf("second RegisterLocal: %v", err)
	}
}

func TestManagerDrainUncordon(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.RegisterLocal()
	if err := mgr.DrainHost("local"); err != nil {
		t.Fatalf("DrainHost: %v", err)
	}
	h, _ := mgr.GetHost("local")
	if h.Status != compute.HostStatusDrained {
		t.Fatalf("expected drained, got %s", h.Status)
	}
	if err := mgr.UncordonHost("local"); err != nil {
		t.Fatalf("UncordonHost: %v", err)
	}
	h, _ = mgr.GetHost("local")
	if h.Status != compute.HostStatusReady {
		t.Fatalf("expected ready, got %s", h.Status)
	}
}

func TestManagerCreateTemplate(t *testing.T) {
	mgr := openManager(t)
	doc := compute.TemplateDoc{Name: "web-small", Image: "web.cap", Resources: compute.ResourceSpec{MemoryBytes: 1 << 30}}
	tmpl, err := mgr.CreateTemplate(doc)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if tmpl.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	got, _ := mgr.GetTemplate("web-small")
	if got.Doc.Resources.MemoryBytes != 1<<30 {
		t.Fatalf("resource not stored: %d", got.Doc.Resources.MemoryBytes)
	}
}

func TestManagerDeleteTemplateBlockedByGroup(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	_, _ = mgr.CreateGroup("asg", "web", 0, 1, 5)
	if err := mgr.DeleteTemplate("web"); err == nil {
		t.Fatal("expected error when template is in use by a group")
	}
}

func TestManagerCreateGroup(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	g, err := mgr.CreateGroup("asg", "web", 1, 2, 5)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if g.TemplateName != "web" {
		t.Fatalf("expected template 'web', got %q", g.TemplateName)
	}
}

func TestManagerCreateGroupDesiredBounds(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	if _, err := mgr.CreateGroup("asg", "web", 3, 1, 5); err == nil {
		t.Fatal("desired < min should fail")
	}
	if _, err := mgr.CreateGroup("asg", "web", 0, 10, 5); err == nil {
		t.Fatal("desired > max should fail")
	}
}

func TestManagerScaleGroup(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	_, _ = mgr.CreateGroup("asg", "web", 0, 1, 10)
	if err := mgr.ScaleGroup("asg", 5); err != nil {
		t.Fatalf("ScaleGroup: %v", err)
	}
	g, _ := mgr.GetGroup("asg")
	if g.DesiredSize != 5 {
		t.Fatalf("expected 5, got %d", g.DesiredSize)
	}
}

func TestManagerRunFromTemplate(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{
		Name:      "web-small",
		Image:     "web.cap",
		Resources: compute.ResourceSpec{MemoryBytes: 512 * 1024 * 1024},
	})
	spec, err := mgr.RunFromTemplate("web-small", "")
	if err != nil {
		t.Fatalf("RunFromTemplate: %v", err)
	}
	if spec.Image != "web.cap" {
		t.Fatalf("expected image web.cap, got %s", spec.Image)
	}
	if spec.Resources.MemoryBytes != 512*1024*1024 {
		t.Fatalf("resources not propagated: %+v", spec.Resources)
	}
	if spec.InstanceName == "" {
		t.Fatal("instance name should be auto-generated")
	}
}

func TestManagerRunFromTemplateCustomName(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	spec, _ := mgr.RunFromTemplate("web", "my-instance")
	if spec.InstanceName != "my-instance" {
		t.Fatalf("expected my-instance, got %s", spec.InstanceName)
	}
}

func TestManagerRunFromTemplateMissing(t *testing.T) {
	mgr := openManager(t)
	if _, err := mgr.RunFromTemplate("nonexistent", ""); err == nil {
		t.Fatal("expected error for missing template")
	}
}

// ---- reconcile --------------------------------------------------------------

func TestManagerReconcileScalesUp(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	_, _ = mgr.CreateGroup("asg", "web", 0, 3, 5)

	var launched []string
	runFn := func(image string, res compute.ResourceSpec, name string) (string, error) {
		id := "inst_" + name
		launched = append(launched, id)
		return id, nil
	}
	statusFn := func(id string) (compute.InstanceStatus, error) {
		return compute.InstanceStatus{ID: id, Status: "running"}, nil
	}

	result, err := mgr.Reconcile("asg", statusFn, runFn)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(result.Created) != 3 {
		t.Fatalf("expected 3 created, got %d", len(result.Created))
	}
	if result.Actual != 3 {
		t.Fatalf("expected actual=3, got %d", result.Actual)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Second reconcile: all 3 running, nothing to do.
	result2, _ := mgr.Reconcile("asg", statusFn, runFn)
	if len(result2.Created) != 0 {
		t.Fatalf("second reconcile should create nothing, got %d", len(result2.Created))
	}
}

func TestManagerReconcileRemovesStaleRecords(t *testing.T) {
	mgr := openManager(t)
	_, _ = mgr.CreateTemplate(compute.TemplateDoc{Name: "web", Image: "web.cap"})
	g, _ := mgr.CreateGroup("asg", "web", 0, 1, 5)

	// Pre-register a "dead" instance.
	_ = mgr.RegisterGroupInstance(g.Name, "dead-inst")

	runFn := func(image string, res compute.ResourceSpec, name string) (string, error) {
		return "new-inst", nil
	}
	// Status func reports "dead-inst" as stopped.
	statusFn := func(id string) (compute.InstanceStatus, error) {
		return compute.InstanceStatus{ID: id, Status: "stopped"}, nil
	}

	result, err := mgr.Reconcile("asg", statusFn, runFn)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	// dead-inst removed, 1 new created.
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
	if len(result.Created) != 1 {
		t.Fatalf("expected 1 created, got %d", len(result.Created))
	}
}
