package org_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/org"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := org.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestEnsureDefaultCreatesProject verifies that EnsureDefault creates the
// "default" project and is idempotent (safe to call multiple times).
func TestEnsureDefaultCreatesProject(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	if err := s.EnsureDefault(); err != nil {
		t.Fatalf("first EnsureDefault: %v", err)
	}
	if err := s.EnsureDefault(); err != nil {
		t.Fatalf("second EnsureDefault (idempotent): %v", err)
	}

	p, err := s.GetProject(org.DefaultProject)
	if err != nil {
		t.Fatalf("GetProject after EnsureDefault: %v", err)
	}
	if p.Name != org.DefaultProject {
		t.Errorf("expected name %q, got %q", org.DefaultProject, p.Name)
	}
}

// TestInsertAndGetProject verifies the round-trip through InsertProject and GetProject.
func TestInsertAndGetProject(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	p := org.Project{
		ID:     "proj_alpha",
		Name:   "alpha",
		Labels: map[string]string{"team": "infra"},
	}
	if err := s.InsertProject(p); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	// resolve by name
	got, err := s.GetProject("alpha")
	if err != nil {
		t.Fatalf("GetProject by name: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID mismatch: want %q got %q", p.ID, got.ID)
	}
	if got.Labels["team"] != "infra" {
		t.Errorf("label not preserved: %v", got.Labels)
	}

	// resolve by ID
	got2, err := s.GetProject("proj_alpha")
	if err != nil {
		t.Fatalf("GetProject by ID: %v", err)
	}
	if got2.Name != "alpha" {
		t.Errorf("name mismatch: %q", got2.Name)
	}
}

// TestListProjects verifies that all inserted projects are returned.
func TestListProjects(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	if err := s.EnsureDefault(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if err := s.InsertProject(org.Project{ID: "proj_" + name, Name: name}); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 3 { // default + alpha + beta
		t.Errorf("expected 3 projects, got %d", len(projects))
	}
}

// TestDeleteProject verifies normal deletion and guards on the default project.
func TestDeleteProject(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	if err := s.EnsureDefault(); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertProject(org.Project{ID: "proj_temp", Name: "temp"}); err != nil {
		t.Fatal(err)
	}

	// delete a regular project
	if err := s.DeleteProject("temp"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := s.GetProject("temp"); err == nil {
		t.Error("expected error after deletion, got nil")
	}

	// deleting the default project must be refused
	if err := s.DeleteProject(org.DefaultProject); err == nil {
		t.Error("expected error deleting default project, got nil")
	}
}

// TestSetProjectLabels verifies label replacement.
func TestSetProjectLabels(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	if err := s.InsertProject(org.Project{ID: "proj_x", Name: "x"}); err != nil {
		t.Fatal(err)
	}

	labels := map[string]string{"env": "prod", "team": "sre"}
	if err := s.SetProjectLabels("x", labels); err != nil {
		t.Fatalf("SetProjectLabels: %v", err)
	}

	got, _ := s.GetProject("x")
	if got.Labels["env"] != "prod" || got.Labels["team"] != "sre" {
		t.Errorf("labels not applied: %v", got.Labels)
	}
}

// TestInitSchemaIdempotent verifies no error on repeated schema init.
func TestInitSchemaIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := org.InitSchema(db); err != nil {
		t.Errorf("second InitSchema: %v", err)
	}
}

// TestOrgRootUsers verifies org root user lifecycle.
func TestOrgRootUsers(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	o, err := s.CreateOrg("test-org")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	u, err := s.AddOrgRootUser(o.ID, "user_001", "root@example.com")
	if err != nil {
		t.Fatalf("AddOrgRootUser: %v", err)
	}
	if !u.MFARequired {
		t.Error("org root user must require MFA by default")
	}

	users, err := s.ListOrgRootUsers(o.ID)
	if err != nil {
		t.Fatalf("ListOrgRootUsers: %v", err)
	}
	if len(users) != 1 || users[0].Email != "root@example.com" {
		t.Errorf("expected 1 org root user, got %d", len(users))
	}

	if err := s.RemoveOrgRootUser(o.ID, "user_001"); err != nil {
		t.Fatalf("RemoveOrgRootUser: %v", err)
	}
	users, _ = s.ListOrgRootUsers(o.ID)
	if len(users) != 0 {
		t.Errorf("expected 0 org root users after removal, got %d", len(users))
	}
}

// TestAccountRootUsers verifies account root user lifecycle.
func TestAccountRootUsers(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	o, _ := s.CreateOrg("acct-root-test-org")
	a, err := s.CreateAccount(o.ID, "main")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	u, err := s.AddAccountRootUser(o.ID, a.ID, "user_002", "acctroot@example.com")
	if err != nil {
		t.Fatalf("AddAccountRootUser: %v", err)
	}
	if !u.MFARequired {
		t.Error("account root user must require MFA by default")
	}

	users, err := s.ListAccountRootUsers(a.ID)
	if err != nil {
		t.Fatalf("ListAccountRootUsers: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 account root user, got %d", len(users))
	}
}

// TestGuardrails verifies guardrail creation, evaluation, and deletion.
func TestGuardrails(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	o, _ := s.CreateOrg("guardrail-org")

	doc := `{"effect":"deny","actions":["instance:delete","storage:*"]}`
	g, err := s.CreateGuardrail(o.ID, "no-delete", "prevent deletion", doc)
	if err != nil {
		t.Fatalf("CreateGuardrail: %v", err)
	}

	// Denied actions
	if err := s.EvaluateGuardrails(o.ID, "instance:delete"); err == nil {
		t.Error("expected guardrail to deny instance:delete")
	}
	if err := s.EvaluateGuardrails(o.ID, "storage:write"); err == nil {
		t.Error("expected guardrail glob to deny storage:write")
	}
	// Allowed action
	if err := s.EvaluateGuardrails(o.ID, "instance:create"); err != nil {
		t.Errorf("instance:create should pass guardrail, got: %v", err)
	}

	// Disable guardrail — previously denied action should now pass
	if err := s.EnableGuardrail(g.ID, false); err != nil {
		t.Fatalf("EnableGuardrail false: %v", err)
	}
	if err := s.EvaluateGuardrails(o.ID, "instance:delete"); err != nil {
		t.Errorf("disabled guardrail should allow, got: %v", err)
	}

	if err := s.DeleteGuardrail(g.ID); err != nil {
		t.Fatalf("DeleteGuardrail: %v", err)
	}
	gs, _ := s.ListGuardrails(o.ID)
	if len(gs) != 0 {
		t.Errorf("expected 0 guardrails after delete, got %d", len(gs))
	}
}

// TestPrincipalURNFormat verifies the URN helper functions.
func TestPrincipalURNFormat(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{org.OrgRootURN("org1", "u1"), "capper:org:org1:root-user:u1"},
		{org.AccountRootURN("org1", "acct1", "u1"), "capper:org:org1:account:acct1:root-user:u1"},
		{org.UserURN("org1", "acct1", "u1"), "capper:org:org1:account:acct1:user:u1"},
		{org.RoleURN("org1", "acct1", "r1"), "capper:org:org1:account:acct1:role:r1"},
		{org.ServiceAccountURN("org1", "acct1", "sa1"), "capper:org:org1:account:acct1:service-account:sa1"},
		{org.SystemURN("control-plane"), "capper:system:control-plane"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("URN mismatch: want %q got %q", c.want, c.got)
		}
	}
}

// TestOrgSchema verifies that new org fields persist correctly.
func TestOrgSchemaFields(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	o, err := s.CreateOrg("acme-corp")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	if o.Slug != "acme-corp" {
		t.Errorf("slug: want %q got %q", "acme-corp", o.Slug)
	}
	if o.Status != org.OrgStatusActive {
		t.Errorf("status: want %q got %q", org.OrgStatusActive, o.Status)
	}
	if o.Plan != "free" {
		t.Errorf("plan: want free got %q", o.Plan)
	}

	got, err := s.GetOrg(o.ID)
	if err != nil {
		t.Fatalf("GetOrg: %v", err)
	}
	if got.Slug != o.Slug {
		t.Errorf("slug round-trip: want %q got %q", o.Slug, got.Slug)
	}
}

// TestAccountSchemaFields verifies that new account fields persist correctly.
func TestAccountSchemaFields(t *testing.T) {
	db := openTestDB(t)
	s := org.NewStore(db)

	o, _ := s.CreateOrg("schema-test-org")
	a, err := s.CreateAccount(o.ID, "Dev Team")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if a.Slug != "dev-team" {
		t.Errorf("slug: want %q got %q", "dev-team", a.Slug)
	}
	if a.AccountType != org.AccountTypeStandard {
		t.Errorf("type: want %q got %q", org.AccountTypeStandard, a.AccountType)
	}

	got, err := s.GetAccount(a.ID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.AccountType != org.AccountTypeStandard {
		t.Errorf("account_type round-trip: want %q got %q", org.AccountTypeStandard, got.AccountType)
	}
}
