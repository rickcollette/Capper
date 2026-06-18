package iam

import "strings"

// Evaluate determines whether principalType:principalID may perform action on
// resource by inspecting all grants and their attached policies.
//
// Resolution order:
//  1. Collect every grant for the principal (direct + via groups).
//  2. For each grant, load the role and all policies attached to it.
//  3. Evaluate every statement in every policy against (action, resource).
//  4. An explicit "deny" in any policy terminates evaluation immediately.
//  5. If at least one "allow" matched, return allow.
//  6. Default: deny.
//
// Returns the decision and the ID of the policy that determined it ("" on
// default deny).
func (s *Store) Evaluate(principalType, principalID, action, resource string) (string, string, error) {
	grants, err := s.GrantsForPrincipal(principalType, principalID)
	if err != nil {
		return DecisionDeny, "", err
	}

	allowed := false
	allowPolicyID := ""

	for _, grant := range grants {
		// Check resource scope: the grant may be scoped to a specific project.
		if !matchResource(grant.ResourceScope, resource) {
			continue
		}

		role, err := s.GetRole(grant.RoleID)
		if err != nil {
			continue
		}

		for _, policyID := range role.Policies {
			pol, err := s.GetPolicy(policyID)
			if err != nil {
				continue
			}
			for _, stmt := range pol.Statements {
				if !matchesAny(stmt.Actions, action) {
					continue
				}
				if !matchesAny(stmt.Resources, resource) {
					continue
				}
				if stmt.Effect == EffectDeny {
					return DecisionDeny, pol.ID, nil
				}
				if stmt.Effect == EffectAllow {
					allowed = true
					allowPolicyID = pol.ID
				}
			}
		}
	}

	if allowed {
		return DecisionAllow, allowPolicyID, nil
	}
	return DecisionDeny, "", nil
}

// matchesAny reports whether value matches any pattern in patterns.
// Supported pattern forms:
//   - "*"       — matches everything
//   - "foo:*"   — matches any string starting with "foo:"
//   - "foo:bar" — exact match
func matchesAny(patterns []string, value string) bool {
	for _, p := range patterns {
		if matchPattern(p, value) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}
	return pattern == value
}

// matchResource checks whether a resource string satisfies a grant's scope.
// The scope may be "*" (all) or a prefix like "project:default".
func matchResource(scope, resource string) bool {
	if scope == "" || scope == "*" {
		return true
	}
	return strings.HasPrefix(resource, scope)
}
