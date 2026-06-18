package ai

type AgentStatus string

const (
	AgentActive  AgentStatus = "active"
	AgentRevoked AgentStatus = "revoked"
)

type Agent struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Project      string      `json:"project"`
	Model        string      `json:"model"`
	Owner        string      `json:"owner"`
	RoleTemplate string      `json:"roleTemplate,omitempty"`
	Status       AgentStatus `json:"status"`
	CreatedAt    string      `json:"createdAt"`
}

type SessionStatus string

const (
	SessionActive SessionStatus = "active"
	SessionEnded  SessionStatus = "ended"
)

type Session struct {
	ID        string        `json:"id"`
	AgentID   string        `json:"agentId"`
	Project   string        `json:"project"`
	Principal string        `json:"principal"`
	Model     string        `json:"model"`
	Status    SessionStatus `json:"status"`
	StartedAt string        `json:"startedAt"`
	EndedAt   string        `json:"endedAt,omitempty"`
}

type MCPServer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`
	Endpoint  string `json:"endpoint"`
	ToolsJSON string `json:"toolsJson,omitempty"`
	IAMAction string `json:"iamAction,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type ToolCall struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Tool      string `json:"tool"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason,omitempty"`
	CalledAt  string `json:"calledAt"`
}

// ---- Secure AI additions ----------------------------------------------------

// ApprovalStatus values.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"
	ApprovalTimeout  = "timeout"
)

// ApprovalGate is a human-in-the-loop checkpoint before a sensitive AI action.
// The AI session blocks until a human resolves the gate or it times out.
type ApprovalGate struct {
	ID          string `json:"id"`
	SessionID   string `json:"sessionId"`
	AgentName   string `json:"agentName"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
	ReviewerNote string `json:"reviewerNote,omitempty"`
	CreatedAt   string `json:"createdAt"`
	ResolvedAt  string `json:"resolvedAt,omitempty"`
}

// AssumedRole records an AI agent temporarily assuming an IAM role with a
// stated purpose and a bounded expiry. Provides the accountability chain:
// human → session → agent → assumed role → action.
type AssumedRole struct {
	ID         string `json:"id"`
	SessionID  string `json:"sessionId"`
	AgentID    string `json:"agentId"`
	RoleID     string `json:"roleId"`
	RoleName   string `json:"roleName"`
	Purpose    string `json:"purpose"`
	ExpiresAt  string `json:"expiresAt"`
	RevokedAt  string `json:"revokedAt,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// LedgerEntry is an immutable audit record for an AI-originated action.
// Written once; never updated or deleted.
type LedgerEntry struct {
	ID          string `json:"id"`
	SessionID   string `json:"sessionId"`
	AgentID     string `json:"agentId"`
	AgentName   string `json:"agentName"`
	HumanPrincipal string `json:"humanPrincipal"`
	Model       string `json:"model"`
	Tool        string `json:"tool"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Decision    string `json:"decision"` // "allowed" | "denied"
	Reason      string `json:"reason"`
	Timestamp   string `json:"timestamp"`
}

// PolicyCondition is a key/value constraint on an AI policy rule.
type PolicyCondition struct {
	Key   string `json:"key"`
	Op    string `json:"op"`    // "eq", "prefix", "contains"
	Value string `json:"value"`
}

// AIPolicy controls what actions an agent/session may perform.
type AIPolicy struct {
	ID          string            `json:"id"`
	AgentID     string            `json:"agentId"`
	Name        string            `json:"name"`
	Effect      string            `json:"effect"`     // "allow" | "deny"
	Actions     []string          `json:"actions"`    // e.g., ["instance:create"]
	Resources   []string          `json:"resources"`  // e.g., ["instance/*"]
	Conditions  []PolicyCondition `json:"conditions,omitempty"`
	RequireApproval bool          `json:"requireApproval"`
	CreatedAt   string            `json:"createdAt"`
}
