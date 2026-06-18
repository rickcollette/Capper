// Package iam implements deny-by-default authorization for Capper operations.
// Every mutating operation must pass through Authorize before it may proceed.
//
// Principal model: users, groups (sets of users), service accounts.
// Permission model: policies (sets of statements) attached to roles;
//
//	roles granted to principals with an optional resource scope.
//
// Audit: every Authorize call is recorded in iam_audit.
package iam

// PrincipalType classifies the entity making a request.
const (
	PrincipalUser           = "user"
	PrincipalGroup          = "group"
	PrincipalServiceAccount = "service-account"
)

// Decision is the outcome of policy evaluation.
const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)

// Effect values inside a policy statement.
const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

// User is a human principal, typically mapped to a local OS account or an
// external identity (e.g. a Google SSO email).
type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	LocalUser string `json:"localUser,omitempty"`
	// Status is the access lifecycle state: "active", "pending" (awaiting admin
	// approval), or "disabled". Defaults to "active" for legacy/local users.
	Status string `json:"status,omitempty"`
	// Provider is the identity source: "local" (OS/CLI) or "google" (SSO).
	Provider  string   `json:"provider,omitempty"`
	Groups    []string `json:"groups,omitempty"`
	CreatedAt string   `json:"createdAt"`
}

// User status values.
const (
	UserStatusActive   = "active"
	UserStatusPending  = "pending"
	UserStatusDisabled = "disabled"
)

// Group is a named set of users. Policies can be granted to groups.
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	AccountID   string   `json:"accountId,omitempty"`
	Members     []string `json:"members,omitempty"`
	CreatedAt   string   `json:"createdAt"`
}

// Role is a named collection of policies that can be granted to principals.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	AccountID   string   `json:"accountId,omitempty"`
	TrustPolicy string   `json:"trustPolicy,omitempty"`
	Policies    []string `json:"policies,omitempty"`
	CreatedAt   string   `json:"createdAt"`
}

// Policy holds a named set of statements.
type Policy struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description,omitempty"`
	AccountID    string      `json:"accountId,omitempty"`
	DocumentJSON string      `json:"documentJson,omitempty"`
	Managed      bool        `json:"managed"`
	Statements   []Statement `json:"statements"`
	CreatedAt    string      `json:"createdAt"`
	UpdatedAt    string      `json:"updatedAt,omitempty"`
}

// Statement is a single allow-or-deny rule within a policy.
type Statement struct {
	Effect    string   `json:"effect"`    // "allow" | "deny"
	Actions   []string `json:"actions"`   // "instance:run", "image:*", "*"
	Resources []string `json:"resources"` // "project:default/*", "*"
}

// Grant binds a role to a principal with an optional resource scope.
type Grant struct {
	ID            string `json:"id"`
	PrincipalType string `json:"principalType"` // user | group | service-account
	PrincipalID   string `json:"principalId"`
	RoleID        string `json:"roleId"`
	ResourceScope string `json:"resourceScope"` // "*" = all, "project:default" = that project
	CreatedAt     string `json:"createdAt"`
}

// ServiceAccount is a non-human principal for automation and AI agents.
type ServiceAccount struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	AccountID   string   `json:"accountId,omitempty"`
	Project     string   `json:"project"`
	Roles       []string `json:"roles,omitempty"`
	CreatedAt   string   `json:"createdAt"`
}

// Token is a signed bearer credential issued to a principal.
type Token struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	PrincipalType string `json:"principalType"`
	PrincipalID   string `json:"principalId"`
	ExpiresAt     string `json:"expiresAt"`
	CreatedAt     string `json:"createdAt"`
}

// AuditRecord captures every Authorize call outcome.
type AuditRecord struct {
	ID            string `json:"id"`
	PrincipalType string `json:"principalType"`
	PrincipalID   string `json:"principalId"`
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Decision      string `json:"decision"`
	PolicyID      string `json:"policyId,omitempty"`
	Timestamp     string `json:"timestamp"`
}
