package ingress

import (
	"database/sql"
	"fmt"
	"time"
)

// IngressRule routes incoming requests to a backend LB or instance.
type IngressRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`
	Host      string `json:"host"`       // e.g., "app.example.com"
	PathPrefix string `json:"pathPrefix"` // e.g., "/api"
	BackendLB string `json:"backendLb"`  // LB name to forward to
	TLSCert   string `json:"tlsCert,omitempty"`
	RateLimit int    `json:"rateLimit,omitempty"` // requests/min, 0 = unlimited
	CreatedAt string `json:"createdAt"`
}

// Store persists ingress rules.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ingress_rules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			project     TEXT NOT NULL,
			host        TEXT NOT NULL DEFAULT '',
			path_prefix TEXT NOT NULL DEFAULT '/',
			backend_lb  TEXT NOT NULL DEFAULT '',
			tls_cert    TEXT NOT NULL DEFAULT '',
			rate_limit  INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS static_sites (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			project       TEXT NOT NULL,
			host          TEXT NOT NULL DEFAULT '',
			source_bucket TEXT NOT NULL DEFAULT '',
			index_file    TEXT NOT NULL DEFAULT 'index.html',
			not_found     TEXT NOT NULL DEFAULT '404.html',
			tls_cert      TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS waf_rules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			project     TEXT NOT NULL,
			match_type  TEXT NOT NULL DEFAULT 'path',
			match_value TEXT NOT NULL DEFAULT '',
			action      TEXT NOT NULL DEFAULT 'block',
			priority    INTEGER NOT NULL DEFAULT 100,
			created_at  TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// Manager handles ingress rules.
type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager { return &Manager{store: s} }

func (m *Manager) Create(name, project, host, path, backendLB, tlsCert string, rateLimit int) (IngressRule, error) {
	rule := IngressRule{
		ID:         fmt.Sprintf("igr_%d", time.Now().UnixNano()),
		Name:       name, Project: project, Host: host, PathPrefix: path,
		BackendLB:  backendLB, TLSCert: tlsCert, RateLimit: rateLimit,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	_, err := m.store.db.Exec(
		`INSERT INTO ingress_rules (id, name, project, host, path_prefix, backend_lb, tls_cert, rate_limit, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.Project, rule.Host, rule.PathPrefix,
		rule.BackendLB, rule.TLSCert, rule.RateLimit, rule.CreatedAt,
	)
	return rule, err
}

func (m *Manager) List(project string) ([]IngressRule, error) {
	rows, err := m.store.db.Query(
		`SELECT id, name, project, host, path_prefix, backend_lb, tls_cert, rate_limit, created_at
		 FROM ingress_rules WHERE project=? ORDER BY name`, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IngressRule
	for rows.Next() {
		var r IngressRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Project, &r.Host, &r.PathPrefix,
			&r.BackendLB, &r.TLSCert, &r.RateLimit, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (m *Manager) Delete(name, project string) error {
	_, err := m.store.db.Exec(`DELETE FROM ingress_rules WHERE name=? AND project=?`, name, project)
	return err
}

// StaticSite hosts static files from a storage bucket or local path.
type StaticSite struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Project     string `json:"project"`
	Host        string `json:"host"`         // custom domain or subdomain
	SourceBucket string `json:"sourceBucket"` // storage bucket name
	IndexFile   string `json:"indexFile"`    // default: "index.html"
	NotFound    string `json:"notFound"`     // default: "404.html"
	TLSCert     string `json:"tlsCert,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// WAFRule is a basic request-filtering rule.
type WAFRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`
	// Match criteria
	MatchType  string `json:"matchType"`  // "ip", "path", "header", "body"
	MatchValue string `json:"matchValue"` // e.g., "192.168.0.0/16", "/admin/*", "X-Bad-Header"
	// Action
	Action    string `json:"action"`   // "block", "allow", "log"
	Priority  int    `json:"priority"` // lower = evaluated first
	CreatedAt string `json:"createdAt"`
}

// StaticSiteManager manages static site hosting.
type StaticSiteManager struct {
	store *Store
}

func NewStaticSiteManager(s *Store) *StaticSiteManager { return &StaticSiteManager{store: s} }

func (m *StaticSiteManager) CreateSite(name, project, host, bucket, index, notFound, tlsCert string) (StaticSite, error) {
	if index == "" {
		index = "index.html"
	}
	if notFound == "" {
		notFound = "404.html"
	}
	site := StaticSite{
		ID: fmt.Sprintf("site_%d", time.Now().UnixNano()),
		Name: name, Project: project, Host: host, SourceBucket: bucket,
		IndexFile: index, NotFound: notFound, TLSCert: tlsCert,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := m.store.db.Exec(
		`INSERT INTO static_sites (id, name, project, host, source_bucket, index_file, not_found, tls_cert, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		site.ID, site.Name, site.Project, site.Host, site.SourceBucket,
		site.IndexFile, site.NotFound, site.TLSCert, site.CreatedAt,
	)
	return site, err
}

func (m *StaticSiteManager) ListSites(project string) ([]StaticSite, error) {
	rows, err := m.store.db.Query(
		`SELECT id, name, project, host, source_bucket, index_file, not_found, tls_cert, created_at
		 FROM static_sites WHERE project=? ORDER BY name`, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaticSite
	for rows.Next() {
		var s StaticSite
		if err := rows.Scan(&s.ID, &s.Name, &s.Project, &s.Host, &s.SourceBucket,
			&s.IndexFile, &s.NotFound, &s.TLSCert, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (m *StaticSiteManager) DeleteSite(name, project string) error {
	_, err := m.store.db.Exec(`DELETE FROM static_sites WHERE name=? AND project=?`, name, project)
	return err
}

// WAFManager manages WAF rules.
type WAFManager struct {
	store *Store
}

func NewWAFManager(s *Store) *WAFManager { return &WAFManager{store: s} }

func (m *WAFManager) CreateRule(name, project, matchType, matchValue, action string, priority int) (WAFRule, error) {
	rule := WAFRule{
		ID: fmt.Sprintf("waf_%d", time.Now().UnixNano()),
		Name: name, Project: project, MatchType: matchType, MatchValue: matchValue,
		Action: action, Priority: priority,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := m.store.db.Exec(
		`INSERT INTO waf_rules (id, name, project, match_type, match_value, action, priority, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.Project, rule.MatchType, rule.MatchValue,
		rule.Action, rule.Priority, rule.CreatedAt,
	)
	return rule, err
}

func (m *WAFManager) ListRules(project string) ([]WAFRule, error) {
	rows, err := m.store.db.Query(
		`SELECT id, name, project, match_type, match_value, action, priority, created_at
		 FROM waf_rules WHERE project=? ORDER BY priority`, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WAFRule
	for rows.Next() {
		var r WAFRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Project, &r.MatchType, &r.MatchValue,
			&r.Action, &r.Priority, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (m *WAFManager) DeleteRule(name, project string) error {
	_, err := m.store.db.Exec(`DELETE FROM waf_rules WHERE name=? AND project=?`, name, project)
	return err
}

// EvaluateRequest checks a request against WAF rules. Returns (action, ruleName).
// action is "allow" if no blocking rule matched.
func (m *WAFManager) EvaluateRequest(project, path, sourceIP string, headers map[string]string) (string, string) {
	rules, err := m.ListRules(project)
	if err != nil || len(rules) == 0 {
		return "allow", ""
	}
	for _, r := range rules {
		matched := false
		switch r.MatchType {
		case "ip":
			matched = sourceIP == r.MatchValue
		case "path":
			matched = matchGlob(path, r.MatchValue)
		case "header":
			for k, v := range headers {
				if k == r.MatchValue || v == r.MatchValue {
					matched = true
					break
				}
			}
		}
		if matched {
			return r.Action, r.Name
		}
	}
	return "allow", ""
}

func matchGlob(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		return len(s) >= len(pattern)-1 && s[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return s == pattern
}
