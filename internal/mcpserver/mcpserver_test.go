package mcpserver

import (
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := NewStore(db)
	if err := s.InitSchema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

func seedServerWithTool(t *testing.T, s *Store, tool Tool) (Server, Tool) {
	t.Helper()
	srv, err := s.CreateServer(Server{Project: "prod", Name: "admin-tools", Runtime: "mcp-go"})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	tool.ServerID = srv.ID
	if tool.IAMAction == "" {
		tool.IAMAction = "mcp:invoke"
	}
	tl, err := s.UpsertTool(tool)
	if err != nil {
		t.Fatalf("upsert tool: %v", err)
	}
	return srv, tl
}

func TestToolInvokeAllowed(t *testing.T) {
	s := newStore(t)
	srv, _ := seedServerWithTool(t, s, Tool{Name: "list_instances", ReadOnly: true, Enabled: true})
	mgr := NewManager(s)

	allow := func(action, resource string) error { return nil }
	res, err := mgr.InvokeTool(srv, "list_instances", []byte("{}"), CallContext{Principal: "agent-1"}, allow)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res.Decision != DecisionAllow {
		t.Errorf("expected allow, got %q", res.Decision)
	}
}

func TestToolInvokeDeniedByIAM(t *testing.T) {
	s := newStore(t)
	srv, _ := seedServerWithTool(t, s, Tool{Name: "delete_all", Enabled: true})
	mgr := NewManager(s)

	deny := func(action, resource string) error { return errors.New("not allowed") }
	res, _ := mgr.InvokeTool(srv, "delete_all", []byte("{}"), CallContext{Principal: "agent-1"}, deny)
	if res.Decision != DecisionDeny {
		t.Errorf("expected deny, got %q", res.Decision)
	}
	invs, _ := s.ListInvocations(srv.ID, 10)
	if len(invs) != 1 || invs[0].Decision != DecisionDeny {
		t.Errorf("deny not audited: %+v", invs)
	}
}

func TestDangerousToolRequiresApproval(t *testing.T) {
	s := newStore(t)
	srv, _ := seedServerWithTool(t, s, Tool{Name: "terminate_node", Dangerous: true, Enabled: true})
	mgr := NewManager(s)

	allow := func(action, resource string) error { return nil }
	res, _ := mgr.InvokeTool(srv, "terminate_node", []byte("{}"), CallContext{Principal: "agent-1"}, allow)
	if res.Decision != DecisionNeedApproval {
		t.Fatalf("expected needs-approval for dangerous tool, got %q", res.Decision)
	}
	if res.ApprovalID == "" {
		t.Fatal("expected an approval to be created")
	}

	// Pending approval exists.
	pending, _ := s.ListApprovals(ApprovalPending)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(pending))
	}

	// Approve it → linked invocation becomes pending (ready to run).
	appr, err := mgr.ResolveApproval(res.ApprovalID, ApprovalApproved, "admin", "ok")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if appr.Status != ApprovalApproved {
		t.Errorf("expected approved, got %q", appr.Status)
	}
	if remaining, _ := s.ListApprovals(ApprovalPending); len(remaining) != 0 {
		t.Errorf("expected 0 pending after approval, got %d", len(remaining))
	}
}

func TestApprovalPolicyAll(t *testing.T) {
	s := newStore(t)
	srv, err := s.CreateServer(Server{Project: "prod", Name: "strict", Runtime: "mcp-go", ApprovalPolicy: "all"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, _ = s.UpsertTool(Tool{ServerID: srv.ID, Name: "read_thing", ReadOnly: true, Enabled: true, IAMAction: "mcp:read"})
	mgr := NewManager(s)
	allow := func(action, resource string) error { return nil }
	// Even a read-only tool needs approval under policy "all".
	res, _ := mgr.InvokeTool(srv, "read_thing", []byte("{}"), CallContext{Principal: "a"}, allow)
	if res.Decision != DecisionNeedApproval {
		t.Errorf("policy=all: expected needs-approval, got %q", res.Decision)
	}
}
