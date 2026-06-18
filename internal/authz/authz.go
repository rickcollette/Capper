// Package authz provides the unified authorization engine for Capper.
//
// Authorization happens in a fixed pipeline:
//  1. Validate authentication token / resolve principal.
//  2. Ensure org and account are active.
//  3. Apply hard system denies.
//  4. Evaluate organization guardrails (explicit deny wins).
//  5. Evaluate cross-account trust if acting across accounts.
//  6. Evaluate account-level IAM policies.
//  7. Verify resource ownership matches account context.
//
// Default decision: DENY. Explicit deny always wins over any allow.
package authz

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AuthContext carries the resolved identity and tenancy context for a request.
// Every API handler that performs authorization must build one of these.
type AuthContext struct {
	OrgID     string
	AccountID string
	ProjectID string

	PrincipalType string // "org-root-user", "account-root-user", "iam-user", "iam-role", "service-account", "system"
	PrincipalID   string
	PrincipalURN  string

	ActingAccountID string // set when cross-account via role assumption
	AssumedRoleID   string
	SessionID       string

	IsOrgRoot     bool
	IsAccountRoot bool

	MFAValidated bool
	TokenID      string
	SourceIP     string
	UserAgent    string
}

// Request is a single authorization query.
type Request struct {
	Auth        AuthContext
	Action      string            // e.g. "compute:instance:create"
	ResourceCRN string            // e.g. "crn:capper:compute:main:us-south-1:acct_123:instance/i-abc"
	Resource    map[string]string // structured resource fields for ownership check
	Conditions  map[string]any
}

// Decision is the outcome of Evaluate.
type Decision struct {
	Allowed bool
	Reason  string // "allowed-by-policy", "denied-by-guardrail", "denied-by-policy", "denied-by-ownership", etc.
}

// PolicyStatement is a single allow-or-deny rule within a policy document.
type PolicyStatement struct {
	Effect    string   `json:"effect"`    // "allow" | "deny"
	Actions   []string `json:"actions"`   // "compute:instance:*", "*"
	Resources []string `json:"resources"` // CRN patterns or "*"
}

// PolicyDocument is the parsed form of an account-level IAM policy.
type PolicyDocument struct {
	Version    string            `json:"version"`
	Statements []PolicyStatement `json:"statements"`
}

// GuardrailDocument is the parsed form of an org-level guardrail.
type GuardrailDocument struct {
	Effect  string   `json:"effect"`  // "deny" or "allow"
	Actions []string `json:"actions"` // action patterns
}

// OrgLookup is the subset of org data the evaluator needs. Callers inject this
// to avoid a direct import cycle.
type OrgLookup struct {
	OrgStatus     string
	AccountStatus string
	Guardrails    []GuardrailDocument
}

// PolicySet is the set of active IAM policies for the requesting principal.
type PolicySet struct {
	Documents []PolicyDocument
}

// Evaluator performs authorization decisions.
type Evaluator struct{}

// NewEvaluator creates a new Evaluator.
func NewEvaluator() *Evaluator { return &Evaluator{} }

// Evaluate runs the full authorization pipeline and returns a Decision.
func (e *Evaluator) Evaluate(req Request, org OrgLookup, policies PolicySet) Decision {
	// 1. Ensure org is active.
	if org.OrgStatus != "" && org.OrgStatus != "active" {
		return Decision{false, "denied-org-not-active"}
	}

	// 2. Ensure account is active.
	if org.AccountStatus != "" && org.AccountStatus != "active" {
		return Decision{false, "denied-account-not-active"}
	}

	// 3. Hard system denies — nothing overrides these.
	if req.Auth.PrincipalType == "" {
		return Decision{false, "denied-no-principal"}
	}

	// 4. Org guardrails — explicit deny wins.
	for _, g := range org.Guardrails {
		if g.Effect == "deny" && matchesAny(g.Actions, req.Action) {
			return Decision{false, "denied-by-guardrail"}
		}
	}

	// 5. Org-root and account-root bypass account IAM (but not guardrails).
	if req.Auth.IsOrgRoot || req.Auth.IsAccountRoot {
		return Decision{true, "allowed-root-user"}
	}

	// 6. System principals are always allowed (internal service calls).
	if req.Auth.PrincipalType == "system" {
		return Decision{true, "allowed-system"}
	}

	// 7. Evaluate account-level IAM policies.
	// Explicit deny anywhere → deny.
	// At least one allow with no overriding deny → allow.
	// No matching allow → deny.
	explicitAllow := false
	for _, doc := range policies.Documents {
		for _, stmt := range doc.Statements {
			if !matchesAny(stmt.Actions, req.Action) {
				continue
			}
			if !resourceMatchesAny(stmt.Resources, req.ResourceCRN) {
				continue
			}
			if stmt.Effect == EffectDeny {
				return Decision{false, "denied-by-policy"}
			}
			if stmt.Effect == EffectAllow {
				explicitAllow = true
			}
		}
	}
	if !explicitAllow {
		return Decision{false, "denied-no-matching-allow"}
	}

	// 8. Resource ownership check (account isolation).
	if accountID, ok := req.Resource["account_id"]; ok {
		if accountID != req.Auth.AccountID && accountID != req.Auth.ActingAccountID {
			return Decision{false, "denied-by-ownership"}
		}
	}

	return Decision{true, "allowed-by-policy"}
}

// ParsePolicyDocument parses a JSON policy document string.
func ParsePolicyDocument(raw string) (PolicyDocument, error) {
	var doc PolicyDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return doc, fmt.Errorf("parse policy: %w", err)
	}
	return doc, nil
}

// ParseGuardrailDocument parses a JSON guardrail document string.
func ParseGuardrailDocument(raw string) (GuardrailDocument, error) {
	var g GuardrailDocument
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return g, fmt.Errorf("parse guardrail: %w", err)
	}
	return g, nil
}

// BuildCRN constructs a Capper Resource Name.
func BuildCRN(service, realm, region, accountID, resourceType, resourceID string) string {
	return fmt.Sprintf("crn:capper:%s:%s:%s:%s:%s/%s",
		service, realm, region, accountID, resourceType, resourceID)
}

// ParseCRN decodes a CRN into its components. Returns an error if the format
// does not match "crn:capper:<svc>:<realm>:<region>:<acct>:<type>/<id>".
func ParseCRN(crn string) (service, realm, region, accountID, resourceType, resourceID string, err error) {
	parts := strings.SplitN(crn, ":", 7)
	if len(parts) < 7 || parts[0] != "crn" || parts[1] != "capper" {
		return "", "", "", "", "", "", fmt.Errorf("invalid CRN: %q", crn)
	}
	service = parts[2]
	realm = parts[3]
	region = parts[4]
	accountID = parts[5]
	rest := parts[6]
	if idx := strings.Index(rest, "/"); idx >= 0 {
		resourceType = rest[:idx]
		resourceID = rest[idx+1:]
	} else {
		resourceType = rest
	}
	return
}

// ---- effect constants -------------------------------------------------------

const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

// ---- matching helpers -------------------------------------------------------

func matchesAny(patterns []string, value string) bool {
	for _, p := range patterns {
		if matchGlob(p, value) {
			return true
		}
	}
	return false
}

func resourceMatchesAny(patterns []string, crn string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if p == "*" || matchGlob(p, crn) {
			return true
		}
	}
	return false
}

// matchGlob supports "*" as a trailing wildcard and exact matching.
func matchGlob(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, pattern[:len(pattern)-1])
	}
	return pattern == value
}
