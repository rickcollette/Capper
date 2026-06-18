package edge

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CacheRule enables CDN-style caching for a path pattern on an ingress.
type CacheRule struct {
	ID          string `json:"id"`
	IngressName string `json:"ingressName"`
	PathPattern string `json:"pathPattern"` // e.g., "/static/*"
	TTLSeconds  int    `json:"ttlSeconds"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
}

// GatewayRoute is a single route in an API gateway.
type GatewayRoute struct {
	ID          string `json:"id"`
	GatewayName string `json:"gatewayName"`
	Method      string `json:"method"`   // "GET", "POST", "*"
	Path        string `json:"path"`     // e.g., "/v1/users"
	Target      string `json:"target"`   // e.g., "service:users"
	AuthMode    string `json:"authMode"` // "none", "iam-token", "jwt"
	CreatedAt   string `json:"createdAt"`
}

// Store manages edge cache rules and API gateway routes.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS edge_cache_rules (
			id           TEXT PRIMARY KEY,
			ingress_name TEXT NOT NULL,
			path_pattern TEXT NOT NULL,
			ttl_seconds  INTEGER NOT NULL DEFAULT 300,
			enabled      INTEGER NOT NULL DEFAULT 1,
			created_at   TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS api_gateway_routes (
			id           TEXT PRIMARY KEY,
			gateway_name TEXT NOT NULL,
			method       TEXT NOT NULL DEFAULT '*',
			path         TEXT NOT NULL,
			target       TEXT NOT NULL,
			auth_mode    TEXT NOT NULL DEFAULT 'none',
			created_at   TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("edge: schema: %w", err)
		}
	}
	return nil
}

// ---- cache rules ------------------------------------------------------------

func (s *Store) EnableCache(ingressName, pathPattern string, ttlSeconds int) (CacheRule, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	r := CacheRule{
		ID:          fmt.Sprintf("ecr_%d", time.Now().UnixNano()),
		IngressName: ingressName,
		PathPattern: pathPattern,
		TTLSeconds:  ttlSeconds,
		Enabled:     true,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.Exec(
		`INSERT INTO edge_cache_rules (id, ingress_name, path_pattern, ttl_seconds, enabled, created_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		r.ID, r.IngressName, r.PathPattern, r.TTLSeconds, r.CreatedAt,
	)
	return r, err
}

func (s *Store) ListCacheRules(ingressName string) ([]CacheRule, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if ingressName != "" {
		rows, err = s.db.Query(
			`SELECT id, ingress_name, path_pattern, ttl_seconds, enabled, created_at
			 FROM edge_cache_rules WHERE ingress_name=? ORDER BY path_pattern`,
			ingressName,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, ingress_name, path_pattern, ttl_seconds, enabled, created_at
			 FROM edge_cache_rules ORDER BY ingress_name, path_pattern`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CacheRule
	for rows.Next() {
		var r CacheRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.IngressName, &r.PathPattern, &r.TTLSeconds, &enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// PurgeCache marks matching cache rules as disabled (cache invalidation signal).
// Returns the count of rules matched.
func (s *Store) PurgeCache(ingressName, path string) (int, error) {
	rules, err := s.ListCacheRules(ingressName)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range rules {
		if matchCachePattern(path, r.PathPattern) {
			_, err := s.db.Exec(`UPDATE edge_cache_rules SET enabled=0 WHERE id=?`, r.ID)
			if err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

func (s *Store) DeleteCacheRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM edge_cache_rules WHERE id=?`, id)
	return err
}

// ---- api gateway routes -----------------------------------------------------

func (s *Store) AddGatewayRoute(gatewayName, method, path, target, authMode string) (GatewayRoute, error) {
	if method == "" {
		method = "*"
	}
	if authMode == "" {
		authMode = "none"
	}
	r := GatewayRoute{
		ID:          fmt.Sprintf("gwr_%d", time.Now().UnixNano()),
		GatewayName: gatewayName,
		Method:      strings.ToUpper(method),
		Path:        path,
		Target:      target,
		AuthMode:    authMode,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.Exec(
		`INSERT INTO api_gateway_routes (id, gateway_name, method, path, target, auth_mode, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.GatewayName, r.Method, r.Path, r.Target, r.AuthMode, r.CreatedAt,
	)
	return r, err
}

func (s *Store) ListGatewayRoutes(gatewayName string) ([]GatewayRoute, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if gatewayName != "" {
		rows, err = s.db.Query(
			`SELECT id, gateway_name, method, path, target, auth_mode, created_at
			 FROM api_gateway_routes WHERE gateway_name=? ORDER BY path`,
			gatewayName,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, gateway_name, method, path, target, auth_mode, created_at
			 FROM api_gateway_routes ORDER BY gateway_name, path`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GatewayRoute
	for rows.Next() {
		var r GatewayRoute
		if err := rows.Scan(&r.ID, &r.GatewayName, &r.Method, &r.Path, &r.Target, &r.AuthMode, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ResolveRoute finds the best matching gateway route for a method+path.
// Returns the first route whose method and path pattern match.
func (s *Store) ResolveRoute(gatewayName, method, path string) (GatewayRoute, bool, error) {
	routes, err := s.ListGatewayRoutes(gatewayName)
	if err != nil {
		return GatewayRoute{}, false, err
	}
	for _, r := range routes {
		methodOK := r.Method == "*" || strings.EqualFold(r.Method, method)
		pathOK := matchCachePattern(path, r.Path)
		if methodOK && pathOK {
			return r, true, nil
		}
	}
	return GatewayRoute{}, false, nil
}

func (s *Store) DeleteGatewayRoute(id string) error {
	_, err := s.db.Exec(`DELETE FROM api_gateway_routes WHERE id=?`, id)
	return err
}

// ---- helpers ----------------------------------------------------------------

// matchCachePattern returns true if path matches pattern.
// Supports exact match and trailing-wildcard prefix (e.g., "/static/*").
func matchCachePattern(path, pattern string) bool {
	if pattern == "*" || pattern == "/*" {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-1] // strip "*", keep "/"
		return strings.HasPrefix(path, prefix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}
