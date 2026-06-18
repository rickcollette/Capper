package iam

import (
	"fmt"
	"strings"
)

// Managed policy names
const (
	PolicyOrgAdmin            = "CapperOrganizationAdministrator"
	PolicyAccountAdmin        = "CapperAccountAdministrator"
	PolicyReadOnly            = "CapperReadOnlyAccess"
	PolicyIAMReadOnly         = "CapperIAMReadOnly"
	PolicyComputeFullAccess   = "CapperComputeFullAccess"
	PolicyNetworkFullAccess   = "CapperNetworkFullAccess"
	PolicyStorageFullAccess   = "CapperStorageFullAccess"
	PolicyKMSPowerUser        = "CapperKMSPowerUser"
	PolicyComputeAdmin        = "CapperComputeAdministrator"
	PolicyNetworkAdmin        = "CapperNetworkAdministrator"
	PolicyStorageAdmin        = "CapperStorageAdministrator"
	PolicyBillingReadOnly     = "CapperBillingReadOnly"
	PolicyAuditReadOnly       = "CapperAuditReadOnly"
	PolicyDeveloperPowerUser  = "CapperDeveloperPowerUser"
	PolicyVPCMobilityOperator = "CapperVPCMobilityOperator"
)

// managedPolicies defines all built-in managed policies.
var managedPolicies = []struct {
	Name        string
	Description string
	Actions     []string
}{
	{
		PolicyOrgAdmin,
		"Full administrative access to all resources in the organization",
		[]string{"*"},
	},
	{
		PolicyAccountAdmin,
		"Full administrative access to all resources within the account",
		[]string{
			"compute:*", "instance:*", "image:*", "network:*", "vpc:*",
			"dns:*", "firewall:*", "lb:*", "ingress:*", "storage:*", "s3:*",
			"kms:*", "secret:*", "certificates:*", "database:*", "stack:*",
			"backup:*", "registry:*", "queue:*", "iam:*", "scheduler:*",
			"node:*", "quota:get", "quota:list", "audit:list", "audit:get",
		},
	},
	{
		PolicyReadOnly,
		"Read-only access to all account resources",
		[]string{
			"*:list", "*:get", "*:describe", "audit:list", "audit:get",
		},
	},
	{
		PolicyIAMReadOnly,
		"Read-only access to IAM resources",
		[]string{"iam:list*", "iam:get*"},
	},
	{
		PolicyComputeFullAccess,
		"Full access to compute, instances, and images",
		[]string{"compute:*", "instance:*", "image:*"},
	},
	{
		PolicyNetworkFullAccess,
		"Full access to networking resources and VPC mobility",
		[]string{
			"network:*", "vpc:*", "vpc:mobility:plan", "vpc:mobility:approve",
			"vpc:mobility:execute", "vpc:mobility:cancel", "vpc:mobility:cutover",
			"vpc:mobility:rollback", "dns:*", "firewall:*", "lb:*", "ingress:*", "waf:*",
			"certificates:create", "certificates:read", "certificates:renew",
			"certificates:bind", "certificates:import",
		},
	},
	{
		PolicyStorageFullAccess,
		"Full access to storage resources",
		[]string{"storage:*", "s3:*"},
	},
	{
		PolicyKMSPowerUser,
		"Full KMS access except delete",
		[]string{"kms:create", "kms:get", "kms:list", "kms:update", "kms:encrypt", "kms:decrypt", "kms:generate", "kms:describe"},
	},
	{
		PolicyComputeAdmin,
		"Full administrative access to compute resources and scheduler",
		[]string{"compute:*", "instance:*", "image:*", "scheduler:*"},
	},
	{
		PolicyNetworkAdmin,
		"Full administrative access to all networking",
		[]string{
			"network:*", "vpc:*", "vpc:mobility:*", "dns:*", "firewall:*",
			"lb:*", "ingress:*", "waf:*",
		},
	},
	{
		PolicyStorageAdmin,
		"Full administrative access to storage",
		[]string{"storage:*", "s3:*", "backup:*", "snapshot:*"},
	},
	{
		PolicyBillingReadOnly,
		"Read-only access to billing and quotas",
		[]string{"billing:get", "billing:list", "quota:get", "quota:list"},
	},
	{
		PolicyAuditReadOnly,
		"Read-only access to audit logs",
		[]string{"audit:list", "audit:get"},
	},
	{
		PolicyDeveloperPowerUser,
		"Full access to compute, network, storage, and DNS; no IAM or org admin",
		[]string{
			"compute:*", "instance:*", "image:*", "network:*", "vpc:*",
			"dns:*", "firewall:*", "lb:*", "ingress:*", "storage:*", "s3:*",
			"backup:*", "database:*", "stack:*", "registry:*", "queue:*",
			"scheduler:simulate", "certificates:create", "certificates:read",
			"certificates:renew", "certificates:bind",
		},
	},
	{
		PolicyVPCMobilityOperator,
		"Access to VPC mobility operations (plan, execute, cutover, rollback) but not approve",
		[]string{
			"vpc:mobility:plan", "vpc:mobility:execute", "vpc:mobility:cancel",
			"vpc:mobility:cutover", "vpc:mobility:rollback",
		},
	},
}

// SeedManagedPolicies inserts or updates all built-in managed policies.
// It is idempotent and called from Manager.Bootstrap().
func (m *Manager) SeedManagedPolicies() error {
	for _, p := range managedPolicies {
		actionsJSON := `["` + strings.Join(p.Actions, `","`) + `"]`
		docJSON := fmt.Sprintf(`{"version":"2024-01-01","statements":[{"effect":"allow","actions":%s,"resources":["*"]}]}`, actionsJSON)
		_, err := m.store.db.Exec(`
			INSERT INTO iam_policies (id, account_id, name, description, document_json, managed, created_at, updated_at)
			VALUES (?, 'system', ?, ?, ?, 1, datetime('now'), datetime('now'))
			ON CONFLICT(id) DO UPDATE SET
				document_json=excluded.document_json,
				description=excluded.description,
				updated_at=excluded.updated_at
			WHERE managed=1`,
			"managed_"+p.Name, p.Name, p.Description, docJSON)
		if err != nil {
			return fmt.Errorf("seeding policy %s: %w", p.Name, err)
		}
	}
	return nil
}
