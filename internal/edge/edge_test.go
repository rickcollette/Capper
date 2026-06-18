package edge_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/edge"
)

func newStore(t *testing.T) *edge.Store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := edge.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return edge.NewStore(db)
}

func TestEnableAndListCache(t *testing.T) {
	s := newStore(t)

	rule, err := s.EnableCache("web", "/static/*", 3600)
	if err != nil {
		t.Fatalf("EnableCache: %v", err)
	}
	if !rule.Enabled {
		t.Error("expected rule to be enabled")
	}
	if rule.TTLSeconds != 3600 {
		t.Errorf("TTLSeconds: got %d", rule.TTLSeconds)
	}

	rules, err := s.ListCacheRules("web")
	if err != nil {
		t.Fatalf("ListCacheRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestPurgeCache(t *testing.T) {
	s := newStore(t)
	_, _ = s.EnableCache("web", "/static/*", 300)
	_, _ = s.EnableCache("web", "/api/*", 60)

	n, err := s.PurgeCache("web", "/static/app.js")
	if err != nil {
		t.Fatalf("PurgeCache: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 rule purged, got %d", n)
	}

	rules, _ := s.ListCacheRules("web")
	var disabledCount int
	for _, r := range rules {
		if !r.Enabled {
			disabledCount++
		}
	}
	if disabledCount != 1 {
		t.Errorf("expected 1 disabled rule, got %d", disabledCount)
	}
}

func TestPurgeCacheNoMatch(t *testing.T) {
	s := newStore(t)
	_, _ = s.EnableCache("web", "/static/*", 300)

	n, err := s.PurgeCache("web", "/api/users")
	if err != nil {
		t.Fatalf("PurgeCache: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 purged, got %d", n)
	}
}

func TestDeleteCacheRule(t *testing.T) {
	s := newStore(t)
	rule, _ := s.EnableCache("web", "/img/*", 120)
	if err := s.DeleteCacheRule(rule.ID); err != nil {
		t.Fatalf("DeleteCacheRule: %v", err)
	}
	rules, _ := s.ListCacheRules("web")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestGatewayRouteCRUD(t *testing.T) {
	s := newStore(t)

	r, err := s.AddGatewayRoute("public-api", "GET", "/v1/users", "service:users", "iam-token")
	if err != nil {
		t.Fatalf("AddGatewayRoute: %v", err)
	}
	if r.Method != "GET" || r.AuthMode != "iam-token" {
		t.Errorf("unexpected route: %+v", r)
	}

	routes, err := s.ListGatewayRoutes("public-api")
	if err != nil {
		t.Fatalf("ListGatewayRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	if err := s.DeleteGatewayRoute(r.ID); err != nil {
		t.Fatalf("DeleteGatewayRoute: %v", err)
	}
	routes, _ = s.ListGatewayRoutes("public-api")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after delete")
	}
}

func TestResolveRoute(t *testing.T) {
	s := newStore(t)
	_, _ = s.AddGatewayRoute("gw", "GET", "/v1/users", "service:users", "none")
	_, _ = s.AddGatewayRoute("gw", "*", "/v1/items/*", "service:items", "none")

	r, ok, err := s.ResolveRoute("gw", "GET", "/v1/users")
	if err != nil || !ok {
		t.Fatalf("expected match: err=%v ok=%v", err, ok)
	}
	if r.Target != "service:users" {
		t.Errorf("target: %q", r.Target)
	}

	r2, ok2, _ := s.ResolveRoute("gw", "POST", "/v1/items/42")
	if !ok2 {
		t.Fatal("expected wildcard match")
	}
	if r2.Target != "service:items" {
		t.Errorf("target: %q", r2.Target)
	}

	_, ok3, _ := s.ResolveRoute("gw", "DELETE", "/v1/other")
	if ok3 {
		t.Error("expected no match for unknown path")
	}
}

func TestInitSchemaIdempotent(t *testing.T) {
	db, _ := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()
	if err := edge.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := edge.InitSchema(db); err != nil {
		t.Errorf("second InitSchema: %v", err)
	}
}
