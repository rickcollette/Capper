package iam

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/user"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
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

	// userMu serializes first-sight SSO user creation so the first-user-admin
	// promotion can't race two simultaneous logins into two admins.
	userMu sync.Mutex
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

	// Seed the assignable "member" role (full non-admin access; granted to SSO
	// users at approval time — never granted automatically here).
	if err := m.ensureMemberRole(); err != nil {
		return err
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

// RoleAdmin and RoleMember are the two assignable RBAC roles. Admin is granted
// to the first user (and the local CLI user); member is assigned to approved SSO
// users by an admin. Both use the legacy statements-based policy path that
// Evaluate understands (managed_* policies are document_json-only).
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// memberActions is the member role's allow-list: full access to workload
// resources, but no IAM administration and no organization/account admin.
var memberActions = []string{
	"compute:*", "instance:*", "image:*", "network:*", "vpc:*",
	"dns:*", "firewall:*", "lb:*", "ingress:*", "waf:*",
	"storage:*", "s3:*", "backup:*", "snapshot:*", "database:*",
	"stack:*", "registry:*", "queue:*", "kms:*", "secret:*",
	"certificates:*", "scheduler:*", "node:list", "node:get",
	"quota:get", "quota:list", "audit:list", "audit:get",
	"iam:list*", "iam:get*",
}

// ensureMemberRole idempotently creates the "member" policy + role.
func (m *Manager) ensureMemberRole() error {
	if _, err := m.store.GetPolicy("member"); err != nil {
		if err2 := m.store.InsertPolicy(Policy{
			ID:   "pol_member",
			Name: "member",
			Statements: []Statement{{
				Effect:    EffectAllow,
				Actions:   memberActions,
				Resources: []string{"*"},
			}},
		}); err2 != nil {
			return fmt.Errorf("iam: bootstrap: insert member policy: %w", err2)
		}
	}
	if _, err := m.store.GetRole("member"); err != nil {
		if err2 := m.store.InsertRole(Role{ID: "role_member", Name: "member"}); err2 != nil {
			return fmt.Errorf("iam: bootstrap: insert member role: %w", err2)
		}
	}
	if err := m.store.AttachPolicy("member", "member"); err != nil {
		return fmt.Errorf("iam: bootstrap: attach member policy: %w", err)
	}
	return nil
}

// ErrAccessDenied is returned when an SSO identity has no matching enabled user
// (no self-registration: only admin-provisioned users may sign in).
var ErrAccessDenied = fmt.Errorf("access denied")

// ResolveSSOUser maps a verified SSO email to an existing, active user. There is
// NO auto-registration: an unknown email is rejected, and a pending/disabled
// user is denied until an admin enables them.
func (m *Manager) ResolveSSOUser(email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, ErrAccessDenied
	}
	u, err := m.store.GetUserByEmail(email)
	if err != nil {
		return User{}, ErrAccessDenied
	}
	if u.Status != UserStatusActive {
		return User{}, fmt.Errorf("account %s", u.Status)
	}
	return u, nil
}

// VerifyPassword authenticates a local user by username (name or email) and
// password, returning the user only if active with a matching password.
func (m *Manager) VerifyPassword(username, password string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return User{}, ErrAccessDenied
	}
	u, err := m.store.GetUser(username)
	if err != nil {
		// Allow login by email too.
		if u, err = m.store.GetUserByEmail(username); err != nil {
			return User{}, ErrAccessDenied
		}
	}
	hash, err := m.store.GetPasswordHash(u.ID)
	if err != nil || hash == "" {
		return User{}, ErrAccessDenied
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, ErrAccessDenied
	}
	if u.Status != UserStatusActive {
		return User{}, fmt.Errorf("account %s", u.Status)
	}
	return u, nil
}

// SetPassword sets (or, with empty plaintext, clears) a user's password.
func (m *Manager) SetPassword(idOrName, plaintext string) error {
	if plaintext == "" {
		return m.store.SetPasswordHash(idOrName, "")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return m.store.SetPasswordHash(idOrName, string(hash))
}

// AdminSetPassword sets a user's password and forces them to change it on next
// login (used when an admin creates or resets a local account).
func (m *Manager) AdminSetPassword(idOrName, plaintext string) error {
	if err := m.SetPassword(idOrName, plaintext); err != nil {
		return err
	}
	return m.store.SetMustChangePassword(idOrName, true)
}

// SetOwnPassword changes a user's own password. The current password must match
// when one is already set (a passwordless SSO user may set an initial one). The
// forced-change flag is cleared on success.
func (m *Manager) SetOwnPassword(idOrName, current, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("new password required")
	}
	hash, _ := m.store.GetPasswordHash(idOrName)
	if hash != "" {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
			return ErrAccessDenied
		}
	}
	if err := m.SetPassword(idOrName, newPassword); err != nil {
		return err
	}
	return m.store.SetMustChangePassword(idOrName, false)
}

// SetOwnEmail updates a user's own email address.
func (m *Manager) SetOwnEmail(idOrName, email string) error {
	return m.store.SetEmail(idOrName, strings.ToLower(strings.TrimSpace(email)))
}

// CreateManagedUser creates an admin-provisioned user (no self-registration).
// provider is "local" (password login) or "google" (SSO by email). The user is
// created active; assign roles separately.
func (m *Manager) CreateManagedUser(name, email, provider string) (User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if provider == "" {
		provider = "local"
	}
	if provider == "google" && email == "" {
		return User{}, fmt.Errorf("email required for google users")
	}
	if name == "" {
		name = email
	}
	if name == "" {
		return User{}, fmt.Errorf("name or email required")
	}
	m.userMu.Lock()
	defer m.userMu.Unlock()
	if email != "" {
		if _, err := m.store.GetUserByEmail(email); err == nil {
			return User{}, fmt.Errorf("user with email %s already exists", email)
		}
	}
	u := User{
		ID:       newID("usr"),
		Name:     name,
		Email:    email,
		Provider: provider,
		Status:   UserStatusActive,
	}
	if err := m.store.InsertUser(u); err != nil {
		return User{}, err
	}
	return u, nil
}

// EnsureAdminUser idempotently makes an email an active admin (deploy bootstrap
// so a fresh system has a first administrator without self-registration).
func (m *Manager) EnsureAdminUser(email, provider string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, fmt.Errorf("empty email")
	}
	if provider == "" {
		provider = "google"
	}
	u, err := m.store.GetUserByEmail(email)
	if err != nil {
		if u, err = m.CreateManagedUser(email, email, provider); err != nil {
			return User{}, err
		}
	}
	if u.Status != UserStatusActive {
		_ = m.store.SetUserStatus(u.ID, UserStatusActive)
	}
	if err := m.assignRoleID(PrincipalUser, u.ID, "role_admin"); err != nil {
		return User{}, err
	}
	return u, nil
}

// assignRoleID grants a role (by role ID) to a principal, idempotently.
func (m *Manager) assignRoleID(pt, pid, roleID string) error {
	grants, _ := m.store.GrantsForPrincipal(pt, pid)
	for _, g := range grants {
		if g.RoleID == roleID {
			return nil
		}
	}
	return m.store.InsertGrant(Grant{
		ID:            newID("grn"),
		PrincipalType: pt,
		PrincipalID:   pid,
		RoleID:        roleID,
		ResourceScope: "*",
	})
}

// AssignRole grants a named role (e.g. "admin"/"member") to a user.
func (m *Manager) AssignRole(userID, roleName string) error {
	role, err := m.store.GetRole(roleName)
	if err != nil {
		return err
	}
	return m.assignRoleID(PrincipalUser, userID, role.ID)
}

// RevokeRole removes a named role grant from a user.
func (m *Manager) RevokeRole(userID, roleName string) error {
	role, err := m.store.GetRole(roleName)
	if err != nil {
		return err
	}
	grants, err := m.store.GrantsForPrincipal(PrincipalUser, userID)
	if err != nil {
		return err
	}
	for _, g := range grants {
		if g.RoleID == role.ID {
			if err := m.store.DeleteGrant(g.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// RolesForUser returns the role names currently granted to a user.
func (m *Manager) RolesForUser(userID string) []string {
	grants, err := m.store.GrantsForPrincipal(PrincipalUser, userID)
	if err != nil {
		return nil
	}
	var out []string
	for _, g := range grants {
		if g.PrincipalType != PrincipalUser || g.PrincipalID != userID {
			continue // skip inherited group grants
		}
		if r, err := m.store.GetRole(g.RoleID); err == nil {
			out = append(out, r.Name)
		}
	}
	return out
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
