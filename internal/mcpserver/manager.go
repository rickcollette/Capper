package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Authorizer decides whether the calling principal may perform action on
// resource. It returns nil to allow, or an error to deny. The API layer wires
// this to the IAM evaluator.
type Authorizer func(action, resource string) error

// Manager orchestrates MCP servers and enforces the tool-call safety contract:
// per-tool IAM authorization, approval gating for dangerous tools, and audited
// invocation records.
type Manager struct {
	store *Store
}

// NewManager wraps a Store.
func NewManager(store *Store) *Manager { return &Manager{store: store} }

// Store exposes the underlying store.
func (m *Manager) Store() *Store { return m.store }

// CallContext carries the identity/provenance of a tool call.
type CallContext struct {
	Principal string
	AgentID   string
	SessionID string
	RequestID string
}

// CallResult is returned from InvokeTool.
type CallResult struct {
	InvocationID string `json:"invocationId"`
	Decision     string `json:"decision"`
	Status       string `json:"status"`
	ApprovalID   string `json:"approvalId,omitempty"`
	Result       string `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
}

func argsHash(args []byte) string {
	sum := sha256.Sum256(args)
	return hex.EncodeToString(sum[:])
}

// InvokeTool evaluates and (when permitted) records a tool call. It enforces:
//  1. the tool must exist and be enabled;
//  2. the caller must pass the IAM check for the tool's action (falling back to
//     the server's default action);
//  3. dangerous/approval-required tools (per the server's approval policy) open
//     a pending approval instead of executing, returning needs-approval.
//
// Execution of the tool body itself is performed by the caller once a call is
// allowed; this method returns the decision and an audited invocation record.
func (m *Manager) InvokeTool(srv Server, toolName string, args []byte, cc CallContext, authorize Authorizer) (CallResult, error) {
	tool, err := m.store.GetTool(srv.ID, toolName)
	if err != nil {
		return CallResult{}, fmt.Errorf("tool %q not found on server %q", toolName, srv.Name)
	}
	if !tool.Enabled {
		return CallResult{Decision: DecisionDeny, Status: InvDenied, Error: "tool is disabled"}, nil
	}

	// IAM check.
	action := tool.IAMAction
	if action == "" {
		action = srv.DefaultIAMAction
	}
	if action != "" && authorize != nil {
		resource := tool.ResourcePattern
		if resource == "" {
			resource = "mcp:" + srv.Name + "/" + tool.Name
		}
		if err := authorize(action, resource); err != nil {
			inv, _ := m.store.RecordInvocation(ToolInvocation{
				Project: srv.Project, ServerID: srv.ID, ToolName: tool.Name, Principal: cc.Principal,
				AgentID: cc.AgentID, SessionID: cc.SessionID, RequestID: cc.RequestID,
				ArgumentsHash: argsHash(args), Decision: DecisionDeny, Status: InvDenied,
				Error: "iam denied: " + err.Error(),
			})
			return CallResult{InvocationID: inv.ID, Decision: DecisionDeny, Status: InvDenied,
				Error: "authorization denied"}, nil
		}
	}

	// Approval gating.
	if requiresApproval(srv.ApprovalPolicy, tool) {
		inv, _ := m.store.RecordInvocation(ToolInvocation{
			Project: srv.Project, ServerID: srv.ID, ToolName: tool.Name, Principal: cc.Principal,
			AgentID: cc.AgentID, SessionID: cc.SessionID, RequestID: cc.RequestID,
			ArgumentsHash: argsHash(args), Decision: DecisionNeedApproval, Status: InvAwaitApprove,
		})
		appr, err := m.store.CreateApproval(Approval{
			Project: srv.Project, ServerID: srv.ID, ToolName: tool.Name, Principal: cc.Principal,
			AgentID: cc.AgentID, InvocationID: inv.ID,
		})
		if err != nil {
			return CallResult{}, err
		}
		return CallResult{InvocationID: inv.ID, Decision: DecisionNeedApproval,
			Status: InvAwaitApprove, ApprovalID: appr.ID}, nil
	}

	// Allowed: record an allow decision. The caller executes the tool body and
	// finalizes via Store.FinishInvocation.
	inv, _ := m.store.RecordInvocation(ToolInvocation{
		Project: srv.Project, ServerID: srv.ID, ToolName: tool.Name, Principal: cc.Principal,
		AgentID: cc.AgentID, SessionID: cc.SessionID, RequestID: cc.RequestID,
		ArgumentsHash: argsHash(args), Decision: DecisionAllow, Status: InvPending,
	})
	return CallResult{InvocationID: inv.ID, Decision: DecisionAllow, Status: InvPending}, nil
}

// requiresApproval reports whether a tool call must be approved before running,
// based on the server's approval policy and the tool's flags.
func requiresApproval(policy string, tool Tool) bool {
	if tool.ApprovalRequired {
		return true
	}
	switch policy {
	case "all":
		return true
	case "none":
		return false
	default: // "dangerous-only"
		return tool.Dangerous
	}
}

// ResolveApproval applies an approval decision. When approved, the linked
// invocation is moved to pending (ready to execute); when denied, to denied.
func (m *Manager) ResolveApproval(approvalID, decision, decidedBy, reason string) (Approval, error) {
	if decision != ApprovalApproved && decision != ApprovalDenied {
		return Approval{}, fmt.Errorf("invalid decision %q", decision)
	}
	if err := m.store.DecideApproval(approvalID, decision, decidedBy, reason); err != nil {
		return Approval{}, err
	}
	appr, err := m.store.GetApproval(approvalID)
	if err != nil {
		return Approval{}, err
	}
	if appr.InvocationID != "" {
		if decision == ApprovalApproved {
			_ = m.store.FinishInvocation(appr.InvocationID, InvPending, 0, "", "")
		} else {
			_ = m.store.FinishInvocation(appr.InvocationID, InvDenied, 0, "denied by "+decidedBy, "")
		}
	}
	return appr, nil
}
