package iam

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CrossAccountPolicy grants a principal from a source account limited access
// to resources in a target account. The grantable actions and resource scopes
// mirror the Statement model used in regular IAM policies.
type CrossAccountPolicy struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	SourceAccount string      `json:"sourceAccount"` // the account granting trust
	TargetAccount string      `json:"targetAccount"` // the account being accessed
	PrincipalType string      `json:"principalType"` // "user" | "service-account"
	PrincipalID   string      `json:"principalId"`
	Statements    []Statement `json:"statements"`
	ExpiresAt     string      `json:"expiresAt,omitempty"`
	CreatedAt     string      `json:"createdAt"`
}

// InitCrossAccountSchema creates the cross_account_policies table if absent.
func InitCrossAccountSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS cross_account_policies (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		source_account TEXT NOT NULL,
		target_account TEXT NOT NULL,
		principal_type TEXT NOT NULL,
		principal_id   TEXT NOT NULL,
		statements     TEXT NOT NULL DEFAULT '[]',
		expires_at     TEXT NOT NULL DEFAULT '',
		created_at     TEXT NOT NULL
	)`)
	return err
}

// CreateCrossAccountPolicy stores a new cross-account policy.
func (m *Manager) CreateCrossAccountPolicy(p CrossAccountPolicy) (CrossAccountPolicy, error) {
	if p.ID == "" {
		b := make([]byte, 6)
		_, _ = cryptoRead(b)
		p.ID = "cap_" + hexEnc(b)
	}
	p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	stmts, _ := json.Marshal(p.Statements)
	_, err := m.store.db.Exec(
		`INSERT INTO cross_account_policies
			(id, name, source_account, target_account, principal_type, principal_id, statements, expires_at, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.SourceAccount, p.TargetAccount, p.PrincipalType, p.PrincipalID,
		string(stmts), p.ExpiresAt, p.CreatedAt,
	)
	if err != nil {
		return CrossAccountPolicy{}, fmt.Errorf("iam: cross-account policy: %w", err)
	}
	return p, nil
}

// GetCrossAccountPolicy retrieves a policy by ID.
func (m *Manager) GetCrossAccountPolicy(id string) (CrossAccountPolicy, error) {
	row := m.store.db.QueryRow(
		`SELECT id, name, source_account, target_account, principal_type, principal_id, statements, expires_at, created_at
		 FROM cross_account_policies WHERE id=?`, id)
	return scanCrossAccountPolicy(row)
}

// ListCrossAccountPolicies returns all cross-account policies for an account.
// If account is empty, all policies are returned.
func (m *Manager) ListCrossAccountPolicies(account string) ([]CrossAccountPolicy, error) {
	var rows *sql.Rows
	var err error
	if account == "" {
		rows, err = m.store.db.Query(
			`SELECT id, name, source_account, target_account, principal_type, principal_id, statements, expires_at, created_at
			 FROM cross_account_policies ORDER BY created_at`)
	} else {
		rows, err = m.store.db.Query(
			`SELECT id, name, source_account, target_account, principal_type, principal_id, statements, expires_at, created_at
			 FROM cross_account_policies
			 WHERE source_account=? OR target_account=?
			 ORDER BY created_at`, account, account)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CrossAccountPolicy
	for rows.Next() {
		p, err := scanCrossAccountPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteCrossAccountPolicy removes a policy by ID.
func (m *Manager) DeleteCrossAccountPolicy(id string) error {
	res, err := m.store.db.Exec(`DELETE FROM cross_account_policies WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("iam: cross-account policy %q not found", id)
	}
	return nil
}

// EvaluateCrossAccount checks whether the given principal from sourceAccount may
// perform action on resource in targetAccount. Returns true if any matching
// unexpired policy grants the action.
func (m *Manager) EvaluateCrossAccount(sourceAccount, targetAccount, principalType, principalID, action, resource string) bool {
	policies, err := m.ListCrossAccountPolicies(sourceAccount)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	for _, p := range policies {
		if p.SourceAccount != sourceAccount || p.TargetAccount != targetAccount {
			continue
		}
		if p.PrincipalType != principalType || p.PrincipalID != principalID {
			continue
		}
		if p.ExpiresAt != "" {
			exp, err := time.Parse(time.RFC3339, p.ExpiresAt)
			if err != nil || now.After(exp) {
				continue // expired or unparseable
			}
		}
		for _, stmt := range p.Statements {
			if stmt.Effect != "allow" {
				continue
			}
			if matchesAny(stmt.Actions, action) && matchesAny(stmt.Resources, resource) {
				return true
			}
		}
	}
	return false
}

// ---- helpers ----------------------------------------------------------------

type capRow interface {
	Scan(dest ...any) error
}

func scanCrossAccountPolicy(row capRow) (CrossAccountPolicy, error) {
	var p CrossAccountPolicy
	var stmtsJSON string
	err := row.Scan(&p.ID, &p.Name, &p.SourceAccount, &p.TargetAccount,
		&p.PrincipalType, &p.PrincipalID, &stmtsJSON, &p.ExpiresAt, &p.CreatedAt)
	if err != nil {
		return CrossAccountPolicy{}, fmt.Errorf("iam: scan cross-account policy: %w", err)
	}
	_ = json.Unmarshal([]byte(stmtsJSON), &p.Statements)
	return p, nil
}

func hexEnc(b []byte) string {
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}

func cryptoRead(b []byte) (int, error) {
	return cryptoRandRead(b)
}

var cryptoRandRead = rand.Read
