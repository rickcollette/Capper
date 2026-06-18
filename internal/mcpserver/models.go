// Package mcpserver implements Capper MCP Servers — managed Model Context
// Protocol tool servers. Unlike normal functions, MCP servers expose tools that
// AI agents can call, so every tool carries an IAM action, optional approval
// gating, and per-call audit. This package owns the mcp_servers, mcp_tools,
// mcp_tool_invocations, and mcp_approvals tables.
package mcpserver

// Server is a managed MCP server.
type Server struct {
	ID               string            `json:"id"`
	Project          string            `json:"project"`
	Name             string            `json:"name"`
	Runtime          string            `json:"runtime"`
	Transport        string            `json:"transport"`
	Endpoint         string            `json:"endpoint,omitempty"`
	PackageID        string            `json:"packageId,omitempty"`
	Image            string            `json:"image,omitempty"`
	Command          []string          `json:"command,omitempty"`
	Version          string            `json:"version"`
	Status           string            `json:"status"`
	DefaultIAMAction string            `json:"defaultIamAction,omitempty"`
	MemoryBytes      int64             `json:"memoryBytes"`
	CPUUnits         int               `json:"cpuUnits"`
	TimeoutMS        int               `json:"timeoutMs"`
	Concurrency      int               `json:"concurrency"`
	MinScale         int               `json:"minScale"`
	MaxScale         int               `json:"maxScale"`
	Isolation        string            `json:"isolation"`
	ApprovalPolicy   string            `json:"approvalPolicy"` // none, dangerous-only, all
	Env              map[string]string `json:"env,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
}

// Tool is one callable tool exposed by an MCP server.
type Tool struct {
	ID               string `json:"id"`
	ServerID         string `json:"mcpServerId"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	InputSchemaJSON  string `json:"inputSchema,omitempty"`
	OutputSchemaJSON string `json:"outputSchema,omitempty"`
	IAMAction        string `json:"iamAction,omitempty"`
	ResourcePattern  string `json:"resourcePattern,omitempty"`
	ReadOnly         bool   `json:"readOnly"`
	ApprovalRequired bool   `json:"approvalRequired"`
	Dangerous        bool   `json:"dangerous"`
	Enabled          bool   `json:"enabled"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// ToolInvocation records one tool call attempt and its decision.
type ToolInvocation struct {
	ID            string `json:"id"`
	Project       string `json:"project"`
	ServerID      string `json:"mcpServerId"`
	ToolName      string `json:"toolName"`
	SessionID     string `json:"sessionId,omitempty"`
	AgentID       string `json:"agentId,omitempty"`
	Principal     string `json:"principal,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
	ArgumentsHash string `json:"argumentsHash,omitempty"`
	Decision      string `json:"decision"` // allow, deny, needs-approval
	ApprovalID    string `json:"approvalId,omitempty"`
	Status        string `json:"status"`
	StartedAt     string `json:"startedAt,omitempty"`
	EndedAt       string `json:"endedAt,omitempty"`
	DurationMS    int64  `json:"durationMs"`
	Error         string `json:"error,omitempty"`
	Result        string `json:"result,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

// Approval is a pending/decided approval for a dangerous tool call.
type Approval struct {
	ID           string `json:"id"`
	Project      string `json:"project"`
	ServerID     string `json:"mcpServerId"`
	ToolName     string `json:"toolName"`
	Principal    string `json:"principal,omitempty"`
	AgentID      string `json:"agentId,omitempty"`
	InvocationID string `json:"invocationId,omitempty"`
	Status       string `json:"status"` // pending, approved, denied
	DecidedBy    string `json:"decidedBy,omitempty"`
	Reason       string `json:"reason,omitempty"`
	CreatedAt    string `json:"createdAt"`
	DecidedAt    string `json:"decidedAt,omitempty"`
}

// Decision values.
const (
	DecisionAllow        = "allow"
	DecisionDeny         = "deny"
	DecisionNeedApproval = "needs-approval"

	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"

	StatusCreated   = "created"
	StatusReady     = "ready"
	InvPending      = "pending"
	InvSucceeded    = "succeeded"
	InvFailed       = "failed"
	InvDenied       = "denied"
	InvAwaitApprove = "awaiting-approval"
)
