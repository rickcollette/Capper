package ai_test

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/ai"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := ai.InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *ai.Manager {
	return ai.NewManager(ai.NewStore(openDB(t)))
}

// ---- Agent registry ---------------------------------------------------------

func TestRegisterAndGetAgent(t *testing.T) {
	m := newManager(t)
	a, err := m.RegisterAgent("bot1", "proj1", "claude-sonnet-4-6", "user1", "analyst")
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if a.ID == "" {
		t.Error("ID must be set")
	}
	if a.Status != ai.AgentActive {
		t.Errorf("status: %v", a.Status)
	}

	got, err := m.GetAgent("bot1", "proj1")
	if err != nil {
		t.Fatalf("GetAgent by name: %v", err)
	}
	if got.Model != "claude-sonnet-4-6" {
		t.Errorf("model: %q", got.Model)
	}
}

func TestListAgents(t *testing.T) {
	m := newManager(t)
	m.RegisterAgent("a1", "proj1", "model1", "u1", "")
	m.RegisterAgent("a2", "proj1", "model2", "u1", "")
	list, err := m.ListAgents("proj1")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d agents, want 2", len(list))
	}
}

func TestRevokeAgent(t *testing.T) {
	m := newManager(t)
	m.RegisterAgent("bot1", "proj1", "m", "u", "")
	if err := m.RevokeAgent("bot1", "proj1"); err != nil {
		t.Fatalf("RevokeAgent: %v", err)
	}
	a, _ := m.GetAgent("bot1", "proj1")
	if a.Status != ai.AgentRevoked {
		t.Errorf("expected revoked, got %v", a.Status)
	}
}

func TestRegisterAgent_RequiresName(t *testing.T) {
	m := newManager(t)
	if _, err := m.RegisterAgent("", "proj1", "m", "u", ""); err == nil {
		t.Error("expected error for empty name")
	}
}

// ---- Session lifecycle ------------------------------------------------------

func TestSessionLifecycle(t *testing.T) {
	m := newManager(t)
	agent, _ := m.RegisterAgent("bot1", "proj1", "model", "user", "")

	sess, err := m.StartSession(agent.ID, "proj1", "alice", "model")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if sess.Status != ai.SessionActive {
		t.Errorf("expected active session, got %v", sess.Status)
	}

	sessions, err := m.ListSessions("proj1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if err := m.EndSession(sess.ID); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	sessions, _ = m.ListSessions("proj1")
	if sessions[0].Status != ai.SessionEnded {
		t.Errorf("expected ended status, got %v", sessions[0].Status)
	}
}

// ---- MCP server registry ----------------------------------------------------

func TestMCPRegistry(t *testing.T) {
	m := newManager(t)
	srv, err := m.RegisterMCP("tools", "proj1", "https://tools.example.com", "ai:tool:call")
	if err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}
	if srv.Endpoint != "https://tools.example.com" {
		t.Errorf("endpoint: %q", srv.Endpoint)
	}

	list, err := m.ListMCP("proj1")
	if err != nil {
		t.Fatalf("ListMCP: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 MCP, got %d", len(list))
	}

	if err := m.DeleteMCP("tools", "proj1"); err != nil {
		t.Fatalf("DeleteMCP: %v", err)
	}
	list, _ = m.ListMCP("proj1")
	if len(list) != 0 {
		t.Errorf("expected 0 MCP after delete")
	}
}

// ---- Tool broker ------------------------------------------------------------

func TestAuthorizeToolCall_NoPolicy(t *testing.T) {
	m := newManager(t)
	// Without any explicit denial policy, AuthorizeToolCall should allow.
	allowed, reason := m.AuthorizeToolCall("sess1", "read_file", "storage:object:get", "/data/file.txt")
	if !allowed {
		t.Errorf("expected allowed, got denied: %s", reason)
	}
}

func TestRecordAndListToolCalls(t *testing.T) {
	m := newManager(t)
	agent, _ := m.RegisterAgent("bot1", "proj1", "m", "u", "")
	sess, _ := m.StartSession(agent.ID, "proj1", "alice", "m")

	if err := m.RecordToolCall(sess.ID, "read_file", "storage:object:get", "/data", "allow", ""); err != nil {
		t.Fatalf("RecordToolCall: %v", err)
	}
	if err := m.RecordToolCall(sess.ID, "write_file", "storage:object:put", "/data/out", "deny", "quota exceeded"); err != nil {
		t.Fatalf("RecordToolCall write: %v", err)
	}

	calls, err := m.ListToolCalls(sess.ID)
	if err != nil {
		t.Fatalf("ListToolCalls: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
}

// ---- Resource escalation guard ----------------------------------------------

func TestBlocksResourceEscalation(t *testing.T) {
	m := newManager(t)
	// gpu:assign is an explicit escalation action blocked by the policy.
	blocked, reason := m.BlocksResourceEscalation("sess1", "gpu:assign")
	if !blocked {
		t.Errorf("expected escalation to be blocked for gpu:assign, reason: %s", reason)
	}
}

func TestNoEscalationForSafeAction(t *testing.T) {
	m := newManager(t)
	blocked, _ := m.BlocksResourceEscalation("sess1", "storage:object:get")
	if blocked {
		t.Error("expected storage:object:get not to be blocked as escalation")
	}
}

// ---- Model gateway ----------------------------------------------------------

func TestModelAllowed(t *testing.T) {
	m := newManager(t)
	// Use a project unique to this test to avoid global allowedModels pollution.
	m.RegisterModel("model-allowed-proj", "claude-sonnet-4-6", 200000)

	if !m.ModelAllowed("model-allowed-proj", "claude-sonnet-4-6") {
		t.Error("expected claude-sonnet-4-6 to be allowed")
	}
	// Unregistered model for this project (but models registered means closed policy).
	if m.ModelAllowed("model-allowed-proj", "gpt-99") {
		t.Error("expected gpt-99 not to be allowed (not registered)")
	}
}

func TestListAllowedModels(t *testing.T) {
	m := newManager(t)
	// Use a project name unique to this test to avoid global-state pollution.
	m.RegisterModel("list-models-proj", "model-a", 100)
	m.RegisterModel("list-models-proj", "model-b", 200)
	models := m.ListAllowedModels("list-models-proj")
	if len(models) < 2 {
		t.Errorf("expected at least 2 models for list-models-proj, got %d", len(models))
	}
}

// ---- Data access policy -----------------------------------------------------

func TestDataAccessPolicy(t *testing.T) {
	m := newManager(t)
	m.SetDataAccessPolicy("sess1", []ai.DataClass{ai.DataClassPublic, ai.DataClassInternal})

	if !m.DataAccessAllowed("sess1", ai.DataClassPublic) {
		t.Error("expected public data access allowed")
	}
	if m.DataAccessAllowed("sess1", ai.DataClassSecret) {
		t.Error("expected secret data access denied")
	}
}

// ---- Agent memory -----------------------------------------------------------

func TestAgentMemory(t *testing.T) {
	m := newManager(t)
	m.WriteMemory("agent1", "task", "analyze logs", ai.MemoryScopeSession, time.Hour)

	val, ok := m.ReadMemory("agent1", "task", ai.MemoryScopeSession)
	if !ok {
		t.Fatal("expected memory entry found")
	}
	if val != "analyze logs" {
		t.Errorf("value: %q", val)
	}
}

func TestPurgeSessionMemory(t *testing.T) {
	m := newManager(t)
	m.WriteMemory("agent1", "k1", "v1", ai.MemoryScopeSession, time.Hour)
	m.WriteMemory("agent1", "k2", "v2", ai.MemoryScopeAgent, time.Hour)

	m.PurgeSessionMemory("agent1")

	_, ok := m.ReadMemory("agent1", "k1", ai.MemoryScopeSession)
	if ok {
		t.Error("session memory should be purged")
	}
	_, ok = m.ReadMemory("agent1", "k2", ai.MemoryScopeAgent)
	if !ok {
		t.Error("global memory should remain after session purge")
	}
}

// ---- Secure AI tests --------------------------------------------------------

func newStore(t *testing.T) *ai.Store {
	return ai.NewStore(openDB(t))
}

func TestApprovalGateLifecycle(t *testing.T) {
	s := newStore(t)

	g, err := s.CreateApprovalGate(ai.ApprovalGate{
		SessionID: "sess_1",
		AgentName: "deployer",
		Action:    "instance:delete",
		Resource:  "instance/web01",
		Reason:    "user requested teardown",
	})
	if err != nil {
		t.Fatalf("CreateApprovalGate: %v", err)
	}
	if g.Status != ai.ApprovalPending {
		t.Errorf("expected pending status, got %q", g.Status)
	}

	gates, err := s.ListApprovalGates("sess_1", ai.ApprovalPending)
	if err != nil {
		t.Fatalf("ListApprovalGates: %v", err)
	}
	if len(gates) != 1 {
		t.Fatalf("expected 1 pending gate, got %d", len(gates))
	}

	if err := s.ResolveApprovalGate(g.ID, ai.ApprovalApproved, "reviewed and approved"); err != nil {
		t.Fatalf("ResolveApprovalGate: %v", err)
	}

	gates, _ = s.ListApprovalGates("sess_1", ai.ApprovalApproved)
	if len(gates) != 1 || gates[0].ReviewerNote != "reviewed and approved" {
		t.Errorf("expected approved gate with note, got %+v", gates)
	}
}

func TestAssumeRoleAndRevoke(t *testing.T) {
	s := newStore(t)

	ar, err := s.AssumeRole("sess_1", "agent_1", "role_admin", "admin-read", "read audit logs", time.Hour)
	if err != nil {
		t.Fatalf("AssumeRole: %v", err)
	}
	if ar.ExpiresAt == "" {
		t.Error("assumed role must have ExpiresAt")
	}

	roles, err := s.ListAssumedRoles("sess_1")
	if err != nil {
		t.Fatalf("ListAssumedRoles: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 assumed role, got %d", len(roles))
	}

	if err := s.RevokeAssumedRole(ar.ID); err != nil {
		t.Fatalf("RevokeAssumedRole: %v", err)
	}
	roles, _ = s.ListAssumedRoles("sess_1")
	if roles[0].RevokedAt == "" {
		t.Error("revoked role must have RevokedAt set")
	}
}

func TestImmutableLedger(t *testing.T) {
	s := newStore(t)

	e, err := s.AppendLedger(ai.LedgerEntry{
		SessionID:      "sess_1",
		AgentID:        "agent_1",
		AgentName:      "deployer",
		HumanPrincipal: "user/rick",
		Model:          "claude-sonnet-4-6",
		Tool:           "capper-cli",
		Action:         "instance:create",
		Resource:       "instance/web01",
		Decision:       "allowed",
		Reason:         "policy permits",
	})
	if err != nil {
		t.Fatalf("AppendLedger: %v", err)
	}
	if e.ID == "" {
		t.Error("expected non-empty ledger entry ID")
	}

	entries, err := s.QueryLedger("sess_1", 10)
	if err != nil {
		t.Fatalf("QueryLedger: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}
	if entries[0].HumanPrincipal != "user/rick" {
		t.Errorf("human principal: %q", entries[0].HumanPrincipal)
	}
}

func TestAIPolicyEngine(t *testing.T) {
	s := newStore(t)

	// Create an allow policy for compute actions.
	_, err := s.CreateAIPolicy(ai.AIPolicy{
		AgentID:  "agent_1",
		Name:     "allow-compute",
		Effect:   "allow",
		Actions:  []string{"instance:create", "instance:list"},
		Resources: []string{"instance/*"},
	})
	if err != nil {
		t.Fatalf("CreateAIPolicy allow: %v", err)
	}

	// Create a deny policy for instance delete.
	_, err = s.CreateAIPolicy(ai.AIPolicy{
		AgentID:  "agent_1",
		Name:     "deny-delete",
		Effect:   "deny",
		Actions:  []string{"instance:delete"},
		Resources: []string{"*"},
	})
	if err != nil {
		t.Fatalf("CreateAIPolicy deny: %v", err)
	}

	// Allowed action.
	ok, needsApproval, err := s.EvaluateAIPolicy("agent_1", "instance:create", "instance/web01")
	if err != nil {
		t.Fatalf("EvaluateAIPolicy allow: %v", err)
	}
	if !ok {
		t.Error("expected instance:create to be allowed")
	}
	_ = needsApproval

	// Denied action.
	ok, _, err = s.EvaluateAIPolicy("agent_1", "instance:delete", "instance/web01")
	if err == nil {
		t.Error("expected denial error for instance:delete")
	}
	if ok {
		t.Error("expected instance:delete to be denied")
	}

	// Unlisted action defaults to deny.
	ok, _, err = s.EvaluateAIPolicy("agent_1", "vpc:create", "vpc/net01")
	if err != nil {
		t.Fatalf("EvaluateAIPolicy no-match: %v", err)
	}
	if ok {
		t.Error("expected vpc:create to be denied (no matching allow policy)")
	}
}

func TestAIPolicy_RequireApproval(t *testing.T) {
	s := newStore(t)

	_, err := s.CreateAIPolicy(ai.AIPolicy{
		AgentID:         "agent_2",
		Name:            "allow-deploy-with-gate",
		Effect:          "allow",
		Actions:         []string{"instance:create"},
		Resources:       []string{"*"},
		RequireApproval: true,
	})
	if err != nil {
		t.Fatalf("CreateAIPolicy: %v", err)
	}

	ok, needsApproval, err := s.EvaluateAIPolicy("agent_2", "instance:create", "instance/prod01")
	if err != nil {
		t.Fatalf("EvaluateAIPolicy: %v", err)
	}
	if !ok {
		t.Error("expected action allowed")
	}
	if !needsApproval {
		t.Error("expected approval gate to be required")
	}
}
