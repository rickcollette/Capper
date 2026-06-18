package iam_test

import (
	"testing"
	"time"

	"capper/internal/iam"
)

func initCrossAccountSchema(t *testing.T) *iam.Manager {
	t.Helper()
	mgr, _ := openTestManager(t)
	db := mgr.IAMStore().DB()
	if err := iam.InitCrossAccountSchema(db); err != nil {
		t.Fatalf("InitCrossAccountSchema: %v", err)
	}
	return mgr
}

func TestCrossAccountCreateAndList(t *testing.T) {
	mgr := initCrossAccountSchema(t)

	p, err := mgr.CreateCrossAccountPolicy(iam.CrossAccountPolicy{
		Name:          "allow-ci-access",
		SourceAccount: "acct-a",
		TargetAccount: "acct-b",
		PrincipalType: iam.PrincipalUser,
		PrincipalID:   "usr_ci",
		Statements: []iam.Statement{{
			Effect:    "allow",
			Actions:   []string{"instance:run", "image:*"},
			Resources: []string{"project:staging/*"},
		}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	list, err := mgr.ListCrossAccountPolicies("acct-a")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("expected 1 policy, got %d", len(list))
	}
}

func TestCrossAccountGet(t *testing.T) {
	mgr := initCrossAccountSchema(t)

	p, _ := mgr.CreateCrossAccountPolicy(iam.CrossAccountPolicy{
		Name:          "get-test",
		SourceAccount: "src",
		TargetAccount: "tgt",
		PrincipalType: iam.PrincipalServiceAccount,
		PrincipalID:   "sa_bot",
		Statements: []iam.Statement{{
			Effect:    "allow",
			Actions:   []string{"stack:apply"},
			Resources: []string{"*"},
		}},
	})

	got, err := mgr.GetCrossAccountPolicy(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "get-test" {
		t.Errorf("name: got %q want %q", got.Name, "get-test")
	}
	if len(got.Statements) != 1 || got.Statements[0].Actions[0] != "stack:apply" {
		t.Errorf("statements: %+v", got.Statements)
	}
}

func TestCrossAccountDelete(t *testing.T) {
	mgr := initCrossAccountSchema(t)
	p, _ := mgr.CreateCrossAccountPolicy(iam.CrossAccountPolicy{
		Name: "delete-me", SourceAccount: "s", TargetAccount: "t",
		PrincipalType: iam.PrincipalUser, PrincipalID: "u",
	})

	if err := mgr.DeleteCrossAccountPolicy(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := mgr.ListCrossAccountPolicies("")
	for _, pp := range list {
		if pp.ID == p.ID {
			t.Error("policy still present after delete")
		}
	}
}

func TestCrossAccountEvaluate_Allow(t *testing.T) {
	mgr := initCrossAccountSchema(t)
	mgr.CreateCrossAccountPolicy(iam.CrossAccountPolicy{
		Name:          "ci-cross",
		SourceAccount: "acct-prod",
		TargetAccount: "acct-staging",
		PrincipalType: iam.PrincipalUser,
		PrincipalID:   "usr_deploy",
		Statements: []iam.Statement{{
			Effect:    "allow",
			Actions:   []string{"instance:*"},
			Resources: []string{"project:staging/*"},
		}},
	})

	if !mgr.EvaluateCrossAccount("acct-prod", "acct-staging", iam.PrincipalUser, "usr_deploy", "instance:run", "project:staging/anything") {
		t.Error("expected allow for matching cross-account policy")
	}
}

func TestCrossAccountEvaluate_Deny_WrongAccount(t *testing.T) {
	mgr := initCrossAccountSchema(t)
	mgr.CreateCrossAccountPolicy(iam.CrossAccountPolicy{
		Name: "limited", SourceAccount: "A", TargetAccount: "B",
		PrincipalType: iam.PrincipalUser, PrincipalID: "usr_x",
		Statements: []iam.Statement{{Effect: "allow", Actions: []string{"*"}, Resources: []string{"*"}}},
	})

	if mgr.EvaluateCrossAccount("A", "C", iam.PrincipalUser, "usr_x", "instance:run", "*") {
		t.Error("should deny: target account C ≠ B")
	}
}

func TestCrossAccountEvaluate_Expired(t *testing.T) {
	mgr := initCrossAccountSchema(t)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	mgr.CreateCrossAccountPolicy(iam.CrossAccountPolicy{
		Name: "expired", SourceAccount: "A", TargetAccount: "B",
		PrincipalType: iam.PrincipalUser, PrincipalID: "u",
		Statements: []iam.Statement{{Effect: "allow", Actions: []string{"*"}, Resources: []string{"*"}}},
		ExpiresAt:  past,
	})

	if mgr.EvaluateCrossAccount("A", "B", iam.PrincipalUser, "u", "instance:run", "*") {
		t.Error("expired policy should not grant access")
	}
}
