package ingress_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/ingress"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingress.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *ingress.Manager {
	return ingress.NewManager(ingress.NewStore(openDB(t)))
}

// ---- IngressRule ------------------------------------------------------------

func TestCreateAndListIngressRule(t *testing.T) {
	m := newManager(t)
	rule, err := m.Create("api-rule", "proj1", "api.example.com", "/api", "api-lb", "", 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rule.ID == "" {
		t.Error("ID must be set")
	}
	if rule.Host != "api.example.com" {
		t.Errorf("host: %q", rule.Host)
	}

	list, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "api-rule" {
		t.Errorf("List: got %v", list)
	}
}

func TestDeleteIngressRule(t *testing.T) {
	m := newManager(t)
	m.Create("del-rule", "proj1", "h.com", "/", "lb1", "", 0)
	if err := m.Delete("del-rule", "proj1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := m.List("proj1")
	if len(list) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(list))
	}
}

func TestCreateIngressRule_WithTLSAndRateLimit(t *testing.T) {
	m := newManager(t)
	rule, err := m.Create("tls-rule", "proj1", "secure.example.com", "/", "lb1", "cert-pem-data", 100)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rule.TLSCert != "cert-pem-data" {
		t.Errorf("tls cert: %q", rule.TLSCert)
	}
	if rule.RateLimit != 100 {
		t.Errorf("rate limit: %d", rule.RateLimit)
	}
}

// ---- StaticSite -------------------------------------------------------------

func TestStaticSiteCRUD(t *testing.T) {
	db := openDB(t)
	m := ingress.NewStaticSiteManager(ingress.NewStore(db))

	site, err := m.CreateSite("docs", "proj1", "docs.example.com", "docs-bucket", "index.html", "404.html", "")
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	if site.SourceBucket != "docs-bucket" {
		t.Errorf("bucket: %q", site.SourceBucket)
	}

	list, err := m.ListSites("proj1")
	if err != nil {
		t.Fatalf("ListSites: %v", err)
	}
	if len(list) != 1 || list[0].Host != "docs.example.com" {
		t.Errorf("ListSites: %v", list)
	}

	if err := m.DeleteSite("docs", "proj1"); err != nil {
		t.Fatalf("DeleteSite: %v", err)
	}
	list, _ = m.ListSites("proj1")
	if len(list) != 0 {
		t.Errorf("expected 0 sites after delete, got %d", len(list))
	}
}

// ---- WAF --------------------------------------------------------------------

func TestWAFRuleCRUD(t *testing.T) {
	db := openDB(t)
	m := ingress.NewWAFManager(ingress.NewStore(db))

	rule, err := m.CreateRule("block-bots", "proj1", "path", "/admin", "block", 100)
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if rule.Action != "block" {
		t.Errorf("action: %q", rule.Action)
	}

	rules, err := m.ListRules("proj1")
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 WAF rule, got %d", len(rules))
	}

	if err := m.DeleteRule("block-bots", "proj1"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	rules, _ = m.ListRules("proj1")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete")
	}
}

func TestWAFEvaluateRequest_Block(t *testing.T) {
	db := openDB(t)
	m := ingress.NewWAFManager(ingress.NewStore(db))
	m.CreateRule("block-admin", "proj1", "path", "/admin*", "block", 100)

	action, matched := m.EvaluateRequest("proj1", "/admin/settings", "1.2.3.4", nil)
	if action != "block" {
		t.Errorf("action: %q, matched: %q", action, matched)
	}
}

func TestWAFEvaluateRequest_Allow(t *testing.T) {
	db := openDB(t)
	m := ingress.NewWAFManager(ingress.NewStore(db))
	m.CreateRule("block-admin", "proj1", "path", "/admin*", "block", 100)

	action, _ := m.EvaluateRequest("proj1", "/api/data", "1.2.3.4", nil)
	if action != "allow" {
		t.Errorf("expected allow for non-matching path, got %q", action)
	}
}

func TestWAFEvaluateRequest_NoRules(t *testing.T) {
	db := openDB(t)
	m := ingress.NewWAFManager(ingress.NewStore(db))

	action, _ := m.EvaluateRequest("proj1", "/api/data", "1.2.3.4", nil)
	if action != "allow" {
		t.Errorf("expected allow with no rules, got %q", action)
	}
}
