package authz_test

import (
	"testing"

	"capper/internal/authz"
)

func TestAllowedByPolicy(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "iam-user", PrincipalID: "usr_1",
		},
		Action: "compute:instance:list", ResourceCRN: "*",
	}
	policies := authz.PolicySet{Documents: []authz.PolicyDocument{{
		Statements: []authz.PolicyStatement{
			{Effect: "allow", Actions: []string{"compute:instance:*"}, Resources: []string{"*"}},
		},
	}}}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, policies)
	if !d.Allowed {
		t.Fatalf("expected allow, got deny: %s", d.Reason)
	}
}

func TestExplicitDenyWins(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "iam-user", PrincipalID: "usr_1",
		},
		Action: "compute:instance:delete", ResourceCRN: "*",
	}
	policies := authz.PolicySet{Documents: []authz.PolicyDocument{{
		Statements: []authz.PolicyStatement{
			{Effect: "allow", Actions: []string{"compute:instance:*"}, Resources: []string{"*"}},
			{Effect: "deny", Actions: []string{"compute:instance:delete"}, Resources: []string{"*"}},
		},
	}}}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, policies)
	if d.Allowed {
		t.Fatal("expected deny due to explicit deny statement")
	}
	if d.Reason != "denied-by-policy" {
		t.Errorf("reason = %q want denied-by-policy", d.Reason)
	}
}

func TestGuardrailDenyBlocksAccountAdmin(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "account-root-user", IsAccountRoot: true,
		},
		Action: "compute:instance:delete", ResourceCRN: "*",
	}
	org := authz.OrgLookup{
		OrgStatus: "active", AccountStatus: "active",
		Guardrails: []authz.GuardrailDocument{
			{Effect: "deny", Actions: []string{"compute:instance:delete"}},
		},
	}
	d := eval.Evaluate(req, org, authz.PolicySet{})
	if d.Allowed {
		t.Fatal("expected guardrail to block account root")
	}
	if d.Reason != "denied-by-guardrail" {
		t.Errorf("reason = %q want denied-by-guardrail", d.Reason)
	}
}

func TestOrgRootAllowedWithoutPolicy(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "org-root-user", IsOrgRoot: true,
		},
		Action: "org:account:create", ResourceCRN: "*",
	}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, authz.PolicySet{})
	if !d.Allowed {
		t.Fatalf("expected org root to be allowed: %s", d.Reason)
	}
}

func TestSuspendedOrgBlocksAll(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "org-root-user", IsOrgRoot: true,
		},
		Action: "compute:instance:list", ResourceCRN: "*",
	}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "suspended", AccountStatus: "active"}, authz.PolicySet{})
	if d.Allowed {
		t.Fatal("expected suspended org to block all actions")
	}
}

func TestSuspendedAccountBlocksActions(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "iam-user", PrincipalID: "usr_1",
		},
		Action: "compute:instance:list", ResourceCRN: "*",
	}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "suspended"}, authz.PolicySet{})
	if d.Allowed {
		t.Fatal("expected suspended account to block actions")
	}
}

func TestResourceOwnershipIsolation(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "iam-user", PrincipalID: "usr_1",
		},
		Action:      "compute:instance:delete",
		ResourceCRN: "*",
		Resource:    map[string]string{"account_id": "acct_2"},
	}
	policies := authz.PolicySet{Documents: []authz.PolicyDocument{{
		Statements: []authz.PolicyStatement{
			{Effect: "allow", Actions: []string{"compute:instance:*"}, Resources: []string{"*"}},
		},
	}}}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, policies)
	if d.Allowed {
		t.Fatal("expected cross-account resource access to be denied")
	}
	if d.Reason != "denied-by-ownership" {
		t.Errorf("reason = %q want denied-by-ownership", d.Reason)
	}
}

func TestNoPolicyDenied(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "iam-user", PrincipalID: "usr_1",
		},
		Action: "vpc:delete", ResourceCRN: "*",
	}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, authz.PolicySet{})
	if d.Allowed {
		t.Fatal("expected deny when no policies present")
	}
}

func TestSystemPrincipalAlwaysAllowed(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "system",
		},
		Action: "compute:instance:list", ResourceCRN: "*",
	}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, authz.PolicySet{})
	if !d.Allowed {
		t.Fatalf("expected system principal to always be allowed: %s", d.Reason)
	}
}

func TestBuildAndParseCRN(t *testing.T) {
	crn := authz.BuildCRN("compute", "main", "us-south-1", "acct_123", "instance", "i-abc")
	svc, realm, region, acct, rtype, rid, err := authz.ParseCRN(crn)
	if err != nil {
		t.Fatalf("ParseCRN: %v", err)
	}
	if svc != "compute" || realm != "main" || region != "us-south-1" ||
		acct != "acct_123" || rtype != "instance" || rid != "i-abc" {
		t.Errorf("CRN parse mismatch: %q", crn)
	}
}

func TestWildcardActionMatch(t *testing.T) {
	eval := authz.NewEvaluator()
	req := authz.Request{
		Auth: authz.AuthContext{
			OrgID: "org_1", AccountID: "acct_1",
			PrincipalType: "iam-user", PrincipalID: "usr_1",
		},
		Action: "storage:bucket:delete", ResourceCRN: "*",
	}
	policies := authz.PolicySet{Documents: []authz.PolicyDocument{{
		Statements: []authz.PolicyStatement{
			{Effect: "allow", Actions: []string{"storage:*"}, Resources: []string{"*"}},
		},
	}}}
	d := eval.Evaluate(req, authz.OrgLookup{OrgStatus: "active", AccountStatus: "active"}, policies)
	if !d.Allowed {
		t.Fatalf("expected wildcard action to match: %s", d.Reason)
	}
}
