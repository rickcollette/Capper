package org

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Store handles all database operations for the org/project hierarchy.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by an already-initialised database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema creates all org tables and applies additive migrations.
// Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		// Organizations — base table
		`CREATE TABLE IF NOT EXISTS organizations (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL UNIQUE,
			created_at    TEXT NOT NULL
		)`,
		// Accounts — base table
		`CREATE TABLE IF NOT EXISTS accounts (
			id         TEXT PRIMARY KEY,
			org_id     TEXT NOT NULL,
			name       TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id         TEXT PRIMARY KEY,
			account_id TEXT NOT NULL DEFAULT '',
			name       TEXT NOT NULL UNIQUE,
			labels     TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
		// Multi-tenant extensions
		`CREATE TABLE IF NOT EXISTS org_root_users (
			id           TEXT PRIMARY KEY,
			org_id       TEXT NOT NULL,
			user_id      TEXT NOT NULL,
			email        TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'active',
			mfa_required INTEGER NOT NULL DEFAULT 1,
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL,
			UNIQUE(org_id, user_id),
			UNIQUE(org_id, email)
		)`,
		`CREATE TABLE IF NOT EXISTS account_root_users (
			id           TEXT PRIMARY KEY,
			org_id       TEXT NOT NULL,
			account_id   TEXT NOT NULL,
			user_id      TEXT NOT NULL,
			email        TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'active',
			mfa_required INTEGER NOT NULL DEFAULT 1,
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL,
			UNIQUE(account_id, user_id),
			UNIQUE(account_id, email)
		)`,
		`CREATE TABLE IF NOT EXISTS org_guardrails (
			id            TEXT PRIMARY KEY,
			org_id        TEXT NOT NULL,
			name          TEXT NOT NULL,
			description   TEXT NOT NULL DEFAULT '',
			document_json TEXT NOT NULL DEFAULT '{}',
			enabled       INTEGER NOT NULL DEFAULT 1,
			created_at    TEXT NOT NULL,
			updated_at    TEXT NOT NULL,
			UNIQUE(org_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS account_memberships (
			id             TEXT PRIMARY KEY,
			org_id         TEXT NOT NULL,
			account_id     TEXT NOT NULL,
			user_id        TEXT NOT NULL,
			principal_type TEXT NOT NULL,
			status         TEXT NOT NULL DEFAULT 'active',
			created_at     TEXT NOT NULL,
			updated_at     TEXT NOT NULL,
			UNIQUE(account_id, user_id, principal_type)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("org: schema: %w", err)
		}
	}

	// Additive migrations for organizations table.
	migrations := []string{
		`ALTER TABLE organizations ADD COLUMN slug          TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE organizations ADD COLUMN status        TEXT NOT NULL DEFAULT 'active'`,
		`ALTER TABLE organizations ADD COLUMN plan          TEXT NOT NULL DEFAULT 'free'`,
		`ALTER TABLE organizations ADD COLUMN billing_email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE organizations ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE organizations ADD COLUMN updated_at    TEXT NOT NULL DEFAULT ''`,
		// Accounts migrations
		`ALTER TABLE accounts ADD COLUMN slug          TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE accounts ADD COLUMN email         TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE accounts ADD COLUMN status        TEXT NOT NULL DEFAULT 'active'`,
		`ALTER TABLE accounts ADD COLUMN account_type  TEXT NOT NULL DEFAULT 'standard'`,
		`ALTER TABLE accounts ADD COLUMN parent_org_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE accounts ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE accounts ADD COLUMN updated_at    TEXT NOT NULL DEFAULT ''`,
	}
	for _, m := range migrations {
		_, _ = db.Exec(m) // ignore "duplicate column" errors
	}

	return nil
}

// ---- projects ---------------------------------------------------------------

func (s *Store) InsertProject(p Project) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	labels, err := marshalLabels(p.Labels)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO projects(id, account_id, name, labels, created_at) VALUES(?,?,?,?,?)`,
		p.ID, p.AccountID, p.Name, labels, p.CreatedAt,
	)
	return err
}

func (s *Store) GetProject(nameOrID string) (Project, error) {
	row := s.db.QueryRow(
		`SELECT id, account_id, name, labels, created_at FROM projects
		 WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID,
	)
	return scanProject(row)
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(
		`SELECT id, account_id, name, labels, created_at FROM projects ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DeleteProject(nameOrID string) error {
	p, err := s.GetProject(nameOrID)
	if err != nil {
		return err
	}
	if p.Name == DefaultProject {
		return fmt.Errorf("cannot delete the %q project", DefaultProject)
	}
	res, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, p.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project %q not found", nameOrID)
	}
	return nil
}

func (s *Store) SetProjectLabels(nameOrID string, labels map[string]string) error {
	p, err := s.GetProject(nameOrID)
	if err != nil {
		return err
	}
	encoded, err := marshalLabels(labels)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE projects SET labels=? WHERE id=?`, encoded, p.ID)
	return err
}

func (s *Store) EnsureDefault() error {
	// Ensure local org exists.
	if _, err := s.GetOrg("org_local"); err != nil {
		now := time.Now().UTC().Format(time.RFC3339)
		_, execErr := s.db.Exec(
			`INSERT INTO organizations (id, slug, name, status, plan, billing_email, metadata_json, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"org_local", "local", "Local", OrgStatusActive, "free", "", "{}", now, now,
		)
		if execErr != nil && !isDuplicateError(execErr) {
			return fmt.Errorf("org: ensure local org: %w", execErr)
		}
	}

	// Ensure local account exists.
	if _, err := s.GetAccount("acct_local"); err != nil {
		now := time.Now().UTC().Format(time.RFC3339)
		_, execErr := s.db.Exec(
			`INSERT INTO accounts (id, org_id, slug, name, email, status, account_type, parent_org_id, metadata_json, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"acct_local", "org_local", "local", "Local", "", "active", AccountTypeManagement, "org_local", "{}", now, now,
		)
		if execErr != nil && !isDuplicateError(execErr) {
			return fmt.Errorf("org: ensure local account: %w", execErr)
		}
	}

	// Ensure default project exists with acct_local set.
	if _, err := s.GetProject(DefaultProject); err != nil {
		if insertErr := s.InsertProject(Project{
			ID:        "proj_default",
			AccountID: "acct_local",
			Name:      DefaultProject,
		}); insertErr != nil {
			return insertErr
		}
	} else {
		// Backfill account_id on the default project if it's empty.
		_, _ = s.db.Exec(
			`UPDATE projects SET account_id='acct_local' WHERE id='proj_default' AND (account_id='' OR account_id IS NULL)`,
		)
	}
	return nil
}

// isDuplicateError returns true for SQLite UNIQUE constraint violations.
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate")
}

// ---- organizations ----------------------------------------------------------

func (s *Store) CreateOrg(name string) (Organization, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	slug := slugify(name)
	o := Organization{
		ID:        "org_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Slug:      slug,
		Name:      name,
		Status:    OrgStatusActive,
		Plan:      "free",
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(
		`INSERT INTO organizations (id, slug, name, status, plan, billing_email, metadata_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.Slug, o.Name, o.Status, o.Plan, "", "{}", o.CreatedAt, o.UpdatedAt,
	)
	return o, err
}

func (s *Store) ListOrgs() ([]Organization, error) {
	rows, err := s.db.Query(
		`SELECT id, COALESCE(slug,''), name, COALESCE(status,'active'), COALESCE(plan,'free'),
		        COALESCE(billing_email,''), COALESCE(metadata_json,'{}'),
		        created_at, COALESCE(updated_at,'')
		 FROM organizations ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Slug, &o.Name, &o.Status, &o.Plan,
			&o.BillingEmail, &o.MetadataJSON, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetOrg(nameOrID string) (Organization, error) {
	var o Organization
	err := s.db.QueryRow(
		`SELECT id, COALESCE(slug,''), name, COALESCE(status,'active'), COALESCE(plan,'free'),
		        COALESCE(billing_email,''), COALESCE(metadata_json,'{}'),
		        created_at, COALESCE(updated_at,'')
		 FROM organizations WHERE id=? OR name=?`, nameOrID, nameOrID,
	).Scan(&o.ID, &o.Slug, &o.Name, &o.Status, &o.Plan,
		&o.BillingEmail, &o.MetadataJSON, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return o, fmt.Errorf("org %q not found", nameOrID)
	}
	return o, nil
}

func (s *Store) UpdateOrg(id string, updates map[string]string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for col, val := range updates {
		if _, err := s.db.Exec(
			`UPDATE organizations SET `+sanitizeCol(col)+`=?, updated_at=? WHERE id=?`,
			val, now, id,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteOrg(nameOrID string) error {
	_, err := s.db.Exec(`DELETE FROM organizations WHERE id=? OR name=?`, nameOrID, nameOrID)
	return err
}

// ---- accounts ---------------------------------------------------------------

func (s *Store) CreateAccount(orgID, name string) (Account, error) {
	return s.CreateAccountTyped(orgID, name, AccountTypeStandard)
}

// CreateAccountTyped creates an account with an explicit account type.
// If accountType is AccountTypeManagement, it enforces that the org has no
// existing management account.
func (s *Store) CreateAccountTyped(orgID, name, accountType string) (Account, error) {
	if accountType == AccountTypeManagement {
		var count int
		s.db.QueryRow(`SELECT count(*) FROM accounts WHERE org_id=? AND account_type='management'`, orgID).Scan(&count)
		if count > 0 {
			return Account{}, fmt.Errorf("organization already has a management account")
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a := Account{
		ID:          "acct_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrgID:       orgID,
		Slug:        slugify(name),
		Name:        name,
		Status:      "active",
		AccountType: accountType,
		ParentOrgID: orgID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := s.db.Exec(
		`INSERT INTO accounts (id, org_id, slug, name, email, status, account_type, parent_org_id, metadata_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.OrgID, a.Slug, a.Name, "", a.Status, a.AccountType, a.ParentOrgID, "{}", a.CreatedAt, a.UpdatedAt,
	)
	return a, err
}

func (s *Store) GetAccount(nameOrID string) (Account, error) {
	var a Account
	err := s.db.QueryRow(
		`SELECT id, org_id, COALESCE(slug,''), name, COALESCE(email,''),
		        COALESCE(status,'active'), COALESCE(account_type,'standard'),
		        COALESCE(parent_org_id,''), COALESCE(metadata_json,'{}'),
		        created_at, COALESCE(updated_at,'')
		 FROM accounts WHERE id=? OR name=?`, nameOrID, nameOrID,
	).Scan(&a.ID, &a.OrgID, &a.Slug, &a.Name, &a.Email,
		&a.Status, &a.AccountType, &a.ParentOrgID, &a.MetadataJSON, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, fmt.Errorf("account %q not found", nameOrID)
	}
	return a, nil
}

func (s *Store) ListAccounts(orgID string) ([]Account, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, COALESCE(slug,''), name, COALESCE(email,''),
		        COALESCE(status,'active'), COALESCE(account_type,'standard'),
		        COALESCE(parent_org_id,''), COALESCE(metadata_json,'{}'),
		        created_at, COALESCE(updated_at,'')
		 FROM accounts WHERE org_id=? ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.OrgID, &a.Slug, &a.Name, &a.Email,
			&a.Status, &a.AccountType, &a.ParentOrgID, &a.MetadataJSON, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---- org root users ---------------------------------------------------------

func (s *Store) AddOrgRootUser(orgID, userID, email string) (OrgRootUser, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	u := OrgRootUser{
		ID:          "orgu_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrgID:       orgID,
		UserID:      userID,
		Email:       email,
		Status:      "active",
		MFARequired: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := s.db.Exec(
		`INSERT INTO org_root_users (id, org_id, user_id, email, status, mfa_required, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.OrgID, u.UserID, u.Email, u.Status, 1, u.CreatedAt, u.UpdatedAt,
	)
	return u, err
}

func (s *Store) ListOrgRootUsers(orgID string) ([]OrgRootUser, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, user_id, email, status, mfa_required, created_at, updated_at
		 FROM org_root_users WHERE org_id=? ORDER BY created_at`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrgRootUser
	for rows.Next() {
		var u OrgRootUser
		var mfa int
		if err := rows.Scan(&u.ID, &u.OrgID, &u.UserID, &u.Email, &u.Status, &mfa, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.MFARequired = mfa != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) RemoveOrgRootUser(orgID, userID string) error {
	_, err := s.db.Exec(`DELETE FROM org_root_users WHERE org_id=? AND user_id=?`, orgID, userID)
	return err
}

// ---- account root users -----------------------------------------------------

func (s *Store) AddAccountRootUser(orgID, accountID, userID, email string) (AccountRootUser, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	u := AccountRootUser{
		ID:          "acru_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrgID:       orgID,
		AccountID:   accountID,
		UserID:      userID,
		Email:       email,
		Status:      "active",
		MFARequired: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := s.db.Exec(
		`INSERT INTO account_root_users (id, org_id, account_id, user_id, email, status, mfa_required, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.OrgID, u.AccountID, u.UserID, u.Email, u.Status, 1, u.CreatedAt, u.UpdatedAt,
	)
	return u, err
}

func (s *Store) ListAccountRootUsers(accountID string) ([]AccountRootUser, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, account_id, user_id, email, status, mfa_required, created_at, updated_at
		 FROM account_root_users WHERE account_id=? ORDER BY created_at`, accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountRootUser
	for rows.Next() {
		var u AccountRootUser
		var mfa int
		if err := rows.Scan(&u.ID, &u.OrgID, &u.AccountID, &u.UserID, &u.Email, &u.Status, &mfa, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.MFARequired = mfa != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAccount(id string, updates map[string]string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	allowed := map[string]bool{
		"status": true, "name": true, "email": true, "account_type": true, "metadata_json": true, "slug": true,
	}
	for col, val := range updates {
		if !allowed[col] {
			continue
		}
		if _, err := s.db.Exec(
			`UPDATE accounts SET `+col+`=?, updated_at=? WHERE id=?`,
			val, now, id,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteAccount(nameOrID string) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE id=? OR name=?`, nameOrID, nameOrID)
	return err
}

func (s *Store) GetGuardrail(id string) (Guardrail, error) {
	var g Guardrail
	var enabled int
	err := s.db.QueryRow(
		`SELECT id, org_id, name, description, document_json, enabled, created_at, updated_at
		 FROM org_guardrails WHERE id=?`, id,
	).Scan(&g.ID, &g.OrgID, &g.Name, &g.Description, &g.DocumentJSON, &enabled, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return g, fmt.Errorf("guardrail %q not found", id)
	}
	g.Enabled = enabled != 0
	return g, nil
}

func (s *Store) RemoveAccountRootUser(accountID, userID string) error {
	_, err := s.db.Exec(`DELETE FROM account_root_users WHERE account_id=? AND user_id=?`, accountID, userID)
	return err
}

// ---- guardrails -------------------------------------------------------------

func (s *Store) CreateGuardrail(orgID, name, description, documentJSON string) (Guardrail, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	g := Guardrail{
		ID:           "grl_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrgID:        orgID,
		Name:         name,
		Description:  description,
		DocumentJSON: documentJSON,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := s.db.Exec(
		`INSERT INTO org_guardrails (id, org_id, name, description, document_json, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.OrgID, g.Name, g.Description, g.DocumentJSON, 1, g.CreatedAt, g.UpdatedAt,
	)
	return g, err
}

func (s *Store) ListGuardrails(orgID string) ([]Guardrail, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, name, description, document_json, enabled, created_at, updated_at
		 FROM org_guardrails WHERE org_id=? ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Guardrail
	for rows.Next() {
		var g Guardrail
		var enabled int
		if err := rows.Scan(&g.ID, &g.OrgID, &g.Name, &g.Description, &g.DocumentJSON, &enabled, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.Enabled = enabled != 0
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) EnableGuardrail(id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE org_guardrails SET enabled=?, updated_at=? WHERE id=?`, v, now, id)
	return err
}

func (s *Store) DeleteGuardrail(id string) error {
	_, err := s.db.Exec(`DELETE FROM org_guardrails WHERE id=?`, id)
	return err
}

// EvaluateGuardrails checks all enabled guardrails for the given org and action.
// Returns an error if any deny guardrail matches the action.
// Guardrail documents are JSON: {"effect":"deny","actions":["instance:create"]}
func (s *Store) EvaluateGuardrails(orgID, action string) error {
	guardrails, err := s.ListGuardrails(orgID)
	if err != nil {
		return err
	}
	for _, g := range guardrails {
		if !g.Enabled {
			continue
		}
		var doc GuardrailDocument
		if err := json.Unmarshal([]byte(g.DocumentJSON), &doc); err != nil {
			continue
		}
		if doc.Effect != "deny" {
			continue
		}
		for _, a := range doc.Actions {
			if a == "*" || a == action || matchesGlob(a, action) {
				return fmt.Errorf("org guardrail %q denies action %q", g.Name, action)
			}
		}
	}
	return nil
}

// ---- account memberships ----------------------------------------------------

func (s *Store) AddMembership(orgID, accountID, userID, principalType string) (AccountMembership, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	m := AccountMembership{
		ID:            "mem_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrgID:         orgID,
		AccountID:     accountID,
		UserID:        userID,
		PrincipalType: principalType,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err := s.db.Exec(
		`INSERT INTO account_memberships (id, org_id, account_id, user_id, principal_type, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.OrgID, m.AccountID, m.UserID, m.PrincipalType, m.Status, m.CreatedAt, m.UpdatedAt,
	)
	return m, err
}

func (s *Store) ListMemberships(accountID string) ([]AccountMembership, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, account_id, user_id, principal_type, status, created_at, updated_at
		 FROM account_memberships WHERE account_id=? ORDER BY created_at`, accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountMembership
	for rows.Next() {
		var m AccountMembership
		if err := rows.Scan(&m.ID, &m.OrgID, &m.AccountID, &m.UserID, &m.PrincipalType, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---- helpers ----------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(s scanner) (Project, error) {
	var p Project
	var labels string
	err := s.Scan(&p.ID, &p.AccountID, &p.Name, &labels, &p.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Project{}, fmt.Errorf("project not found")
		}
		return Project{}, err
	}
	p.Labels, err = unmarshalLabels(labels)
	return p, err
}

func marshalLabels(m map[string]string) (string, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func unmarshalLabels(s string) (map[string]string, error) {
	if s == "" || s == "{}" {
		return nil, nil
	}
	var m map[string]string
	return m, json.Unmarshal([]byte(s), &m)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' {
			b.WriteRune('-')
		}
	}
	return b.String()
}

// sanitizeCol whitelists column names to prevent SQL injection.
func sanitizeCol(col string) string {
	allowed := map[string]bool{
		"status": true, "plan": true, "billing_email": true, "metadata_json": true, "slug": true,
	}
	if allowed[col] {
		return col
	}
	return "status"
}

// matchesGlob does a simple prefix/suffix glob match (e.g., "instance:*").
func matchesGlob(pattern, action string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(action, strings.TrimSuffix(pattern, "*"))
	}
	return false
}
