package iam

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/user"
	"sync"
	"time"
)

// tokenCacheTTL bounds how long a token's existence (non-revocation) is trusted
// without re-querying the store. Verify runs on every authenticated request, so
// this avoids a DB round-trip per call; revocation via RevokeToken is reflected
// immediately, and a direct store delete converges within the TTL.
const tokenCacheTTL = 5 * time.Second

// Manager wraps the Store and provides high-level operations:
// policy evaluation, authorization enforcement, bootstrap, and token issuance.
type Manager struct {
	store  *Store
	sigKey []byte

	// tokenCache memoizes that a token ID still exists in the store, to keep the
	// per-request revocation check off the hot DB path. Only positive existence
	// is cached (bounded by tokenCacheTTL); a missing/revoked token always falls
	// through to a store lookup and is denied.
	tokenMu    sync.Mutex
	tokenCache map[string]time.Time
}

// NewManager creates a Manager. storeRoot is the Capper store directory;
// it is used to locate (or create) the HMAC signing key.
func NewManager(store *Store, storeRoot string) (*Manager, error) {
	key, err := loadOrCreateSigningKey(storeRoot)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store, sigKey: key, tokenCache: map[string]time.Time{}}, nil
}

// tokenExists reports whether tokenID is present in the store, consulting a
// short-lived positive cache first. Used by Verify for the revocation check.
func (m *Manager) tokenExists(tokenID string) bool {
	now := time.Now()
	m.tokenMu.Lock()
	if exp, ok := m.tokenCache[tokenID]; ok && now.Before(exp) {
		m.tokenMu.Unlock()
		return true
	}
	m.tokenMu.Unlock()

	if _, err := m.store.GetToken(tokenID); err != nil {
		return false
	}
	m.tokenMu.Lock()
	m.tokenCache[tokenID] = now.Add(tokenCacheTTL)
	m.tokenMu.Unlock()
	return true
}

// RevokeToken deletes a token and immediately evicts it from the existence
// cache so the revocation takes effect on the next request.
func (m *Manager) RevokeToken(tokenID string) error {
	if err := m.store.DeleteToken(tokenID); err != nil {
		return err
	}
	m.tokenMu.Lock()
	delete(m.tokenCache, tokenID)
	m.tokenMu.Unlock()
	return nil
}

// Store exposes the underlying store for direct queries (e.g. from CLI).
func (m *Manager) IAMStore() *Store { return m.store }

// Authorize checks whether principal may perform action on resource.
// It writes an audit record and returns a non-nil error on denial.
//
// principalType is one of the PrincipalXxx constants; principalID is the
// resource ID of the principal (e.g. the user ID).
func (m *Manager) Authorize(principalType, principalID, action, resource string) error {
	decision, policyID, err := m.store.Evaluate(principalType, principalID, action, resource)
	if err != nil {
		return fmt.Errorf("iam: evaluate: %w", err)
	}

	_ = m.store.InsertAudit(AuditRecord{
		ID:            newID("aud"),
		PrincipalType: principalType,
		PrincipalID:   principalID,
		Action:        action,
		Resource:      resource,
		Decision:      decision,
		PolicyID:      policyID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})

	if decision == DecisionDeny {
		return fmt.Errorf("iam: %s:%s is not allowed to %s on %s", principalType, principalID, action, resource)
	}
	return nil
}

// LocalPrincipal returns the principalType and principalID for the currently
// running OS user. If the user has no IAM record, it returns "user" and the
// raw OS username so callers can check against the bootstrap user.
func (m *Manager) LocalPrincipal() (string, string) {
	osUser, err := user.Current()
	if err != nil {
		return PrincipalUser, "unknown"
	}
	u, err := m.store.GetUserByLocalUser(osUser.Username)
	if err != nil {
		// Not yet registered — return raw OS username as ID (bootstrap path).
		return PrincipalUser, osUser.Username
	}
	return PrincipalUser, u.ID
}

// Bootstrap ensures the admin role and a grant for the local OS user exist.
// It is idempotent and safe to call on every store.Open.
//
// Bootstrap creates:
//   - policy "admin-all" with allow * on *
//   - role "admin" with admin-all attached
//   - user record for the local OS user
//   - grant of admin role to that user
func (m *Manager) Bootstrap() error {
	osUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("iam: bootstrap: get current user: %w", err)
	}

	// Policy: admin-all
	if _, err := m.store.GetPolicy("admin-all"); err != nil {
		if err2 := m.store.InsertPolicy(Policy{
			ID:   "pol_admin_all",
			Name: "admin-all",
			Statements: []Statement{{
				Effect:    EffectAllow,
				Actions:   []string{"*"},
				Resources: []string{"*"},
			}},
		}); err2 != nil {
			return fmt.Errorf("iam: bootstrap: insert admin policy: %w", err2)
		}
	}

	// Role: admin
	if _, err := m.store.GetRole("admin"); err != nil {
		if err2 := m.store.InsertRole(Role{ID: "role_admin", Name: "admin"}); err2 != nil {
			return fmt.Errorf("iam: bootstrap: insert admin role: %w", err2)
		}
	}
	// Ensure policy is attached to role (idempotent via INSERT OR IGNORE).
	if err := m.store.AttachPolicy("admin", "admin-all"); err != nil {
		return fmt.Errorf("iam: bootstrap: attach policy: %w", err)
	}

	// User record for local OS user.
	adminUser, err := m.store.GetUserByLocalUser(osUser.Username)
	if err != nil {
		adminUser = User{
			ID:        newID("usr"),
			Name:      osUser.Username,
			LocalUser: osUser.Username,
		}
		if err2 := m.store.InsertUser(adminUser); err2 != nil {
			return fmt.Errorf("iam: bootstrap: insert user: %w", err2)
		}
	}

	// Grant admin role to that user (idempotent: check first).
	grants, _ := m.store.GrantsForPrincipal(PrincipalUser, adminUser.ID)
	for _, g := range grants {
		if g.RoleID == "role_admin" {
			_ = m.SeedManagedPolicies()
			return nil // already granted
		}
	}
	if err := m.store.InsertGrant(Grant{
		ID:            newID("grn"),
		PrincipalType: PrincipalUser,
		PrincipalID:   adminUser.ID,
		RoleID:        "role_admin",
		ResourceScope: "*",
	}); err != nil {
		return err
	}
	return m.SeedManagedPolicies()
}

// LookupByS3AccessKey finds the principal that owns the given S3 access key.
// Returns (PrincipalServiceAccount or PrincipalUser, principalID, error).
// The lookup joins s3_credentials → account_id, then finds the IAM principal
// whose service-account or user ID matches that account_id.
func (m *Manager) LookupByS3AccessKey(accessKey string) (principalType, principalID string, err error) {
	var accountID string
	row := m.store.DB().QueryRow(
		`SELECT account_id FROM s3_credentials WHERE access_key=?`, accessKey)
	if err := row.Scan(&accountID); err != nil {
		return "", "", fmt.Errorf("iam: LookupByS3AccessKey: access key not found")
	}
	// Try service account first, then user.
	var id string
	if err := m.store.DB().QueryRow(
		`SELECT id FROM iam_service_accounts WHERE id=?`, accountID).Scan(&id); err == nil {
		return PrincipalServiceAccount, accountID, nil
	}
	if err := m.store.DB().QueryRow(
		`SELECT id FROM iam_users WHERE id=?`, accountID).Scan(&id); err == nil {
		return PrincipalUser, accountID, nil
	}
	// Account ID doesn't map to a known principal; treat it as a service account.
	return PrincipalServiceAccount, accountID, nil
}

// newID generates a short random ID with the given prefix.
func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
