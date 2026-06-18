// Package org manages the organization → account → project hierarchy used to
// namespace every Capper resource. In single-user mode the hierarchy is
// transparent: a "default" project is created automatically and the
// --project flag defaults to it.
package org

import "fmt"

// Organization is the top-level billing/tenant boundary.
// In single-user mode exactly one org ("local") is created automatically.
type Organization struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Plan         string `json:"plan"`
	BillingEmail string `json:"billingEmail"`
	MetadataJSON string `json:"metadata,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// Account is a logical sub-unit of an organization (team, department).
// In single-user mode it is omitted; ProjectAccountID is left empty.
type Account struct {
	ID           string `json:"id"`
	OrgID        string `json:"orgId"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	AccountType  string `json:"accountType"`
	ParentOrgID  string `json:"parentOrgId"`
	MetadataJSON string `json:"metadata,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// Project is the primary namespacing unit. Every resource carries a ProjectID.
type Project struct {
	ID        string            `json:"id"`
	AccountID string            `json:"accountId,omitempty"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

// OrgRootUser is a highly-privileged principal that controls an organization.
// Root users bypass account IAM but remain subject to org guardrails.
type OrgRootUser struct {
	ID          string `json:"id"`
	OrgID       string `json:"orgId"`
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	Status      string `json:"status"`
	MFARequired bool   `json:"mfaRequired"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// AccountRootUser is the highly-privileged principal for a single account.
type AccountRootUser struct {
	ID          string `json:"id"`
	OrgID       string `json:"orgId"`
	AccountID   string `json:"accountId"`
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	Status      string `json:"status"`
	MFARequired bool   `json:"mfaRequired"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Guardrail is an organization-level deny/allow constraint evaluated before
// account-level IAM. Guardrails cannot be overridden by account policies.
type Guardrail struct {
	ID           string `json:"id"`
	OrgID        string `json:"orgId"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DocumentJSON string `json:"document"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// GuardrailDocument is the parsed form of a guardrail's JSON document.
type GuardrailDocument struct {
	Effect  string   `json:"effect"`  // "deny" or "allow"
	Actions []string `json:"actions"` // e.g., ["instance:create", "*"]
}

// AccountMembership maps a principal into an account.
type AccountMembership struct {
	ID            string `json:"id"`
	OrgID         string `json:"orgId"`
	AccountID     string `json:"accountId"`
	UserID        string `json:"userId"`
	PrincipalType string `json:"principalType"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

const DefaultProject = "default"

// Org status values.
const (
	OrgStatusActive        = "active"
	OrgStatusSuspended     = "suspended"
	OrgStatusPendingDelete = "pending_delete"
	OrgStatusDeleted       = "deleted"
)

// Account type values.
const (
	AccountTypeManagement  = "management"
	AccountTypeStandard    = "standard"
	AccountTypeSandbox     = "sandbox"
	AccountTypeService     = "service"
	AccountTypeMarketplace = "marketplace"
)

// ---- Principal URN helpers --------------------------------------------------

// OrgRootURN returns the canonical URN for an org root user.
func OrgRootURN(orgID, userID string) string {
	return fmt.Sprintf("capper:org:%s:root-user:%s", orgID, userID)
}

// AccountRootURN returns the canonical URN for an account root user.
func AccountRootURN(orgID, accountID, userID string) string {
	return fmt.Sprintf("capper:org:%s:account:%s:root-user:%s", orgID, accountID, userID)
}

// UserURN returns the canonical URN for an IAM user.
func UserURN(orgID, accountID, userID string) string {
	return fmt.Sprintf("capper:org:%s:account:%s:user:%s", orgID, accountID, userID)
}

// RoleURN returns the canonical URN for an IAM role.
func RoleURN(orgID, accountID, roleID string) string {
	return fmt.Sprintf("capper:org:%s:account:%s:role:%s", orgID, accountID, roleID)
}

// ServiceAccountURN returns the canonical URN for a service account.
func ServiceAccountURN(orgID, accountID, saID string) string {
	return fmt.Sprintf("capper:org:%s:account:%s:service-account:%s", orgID, accountID, saID)
}

// SystemURN returns the canonical URN for an internal system service.
func SystemURN(serviceName string) string {
	return fmt.Sprintf("capper:system:%s", serviceName)
}
