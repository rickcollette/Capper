package ai

import (
	"fmt"
	"strings"
	"time"
)

type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager { return &Manager{store: s} }

// ---- agent registry ---------------------------------------------------------

func (m *Manager) RegisterAgent(name, project, model, owner, roleTemplate string) (Agent, error) {
	if name == "" {
		return Agent{}, fmt.Errorf("ai: agent name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a := Agent{
		ID:           newID("agent"),
		Name:         name,
		Project:      project,
		Model:        model,
		Owner:        owner,
		RoleTemplate: roleTemplate,
		Status:       AgentActive,
		CreatedAt:    now,
	}
	if err := m.store.InsertAgent(a); err != nil {
		return Agent{}, fmt.Errorf("ai: register agent: %w", err)
	}
	return a, nil
}

func (m *Manager) GetAgent(nameOrID, project string) (Agent, error) {
	return m.store.GetAgent(nameOrID, project)
}

func (m *Manager) ListAgents(project string) ([]Agent, error) {
	return m.store.ListAgents(project)
}

func (m *Manager) RevokeAgent(nameOrID, project string) error {
	a, err := m.store.GetAgent(nameOrID, project)
	if err != nil {
		return err
	}
	return m.store.UpdateAgentStatus(a.ID, AgentRevoked)
}

// ---- sessions ---------------------------------------------------------------

func (m *Manager) StartSession(agentID, project, principal, model string) (Session, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	sess := Session{
		ID:        newID("sess"),
		AgentID:   agentID,
		Project:   project,
		Principal: principal,
		Model:     model,
		Status:    SessionActive,
		StartedAt: now,
	}
	if err := m.store.InsertSession(sess); err != nil {
		return Session{}, fmt.Errorf("ai: start session: %w", err)
	}
	return sess, nil
}

func (m *Manager) EndSession(sessionID string) error {
	return m.store.EndSession(sessionID)
}

func (m *Manager) ListSessions(project string) ([]Session, error) {
	return m.store.ListSessions(project)
}

// ---- mcp server registry ----------------------------------------------------

func (m *Manager) RegisterMCP(name, project, endpoint, iamAction string) (MCPServer, error) {
	if name == "" {
		return MCPServer{}, fmt.Errorf("ai: mcp server name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	srv := MCPServer{
		ID:        newID("mcp"),
		Name:      name,
		Project:   project,
		Endpoint:  endpoint,
		IAMAction: iamAction,
		CreatedAt: now,
	}
	if err := m.store.InsertMCP(srv); err != nil {
		return MCPServer{}, fmt.Errorf("ai: register mcp: %w", err)
	}
	return srv, nil
}

func (m *Manager) ListMCP(project string) ([]MCPServer, error) {
	return m.store.ListMCP(project)
}

func (m *Manager) DeleteMCP(nameOrID, project string) error {
	return m.store.DeleteMCP(nameOrID, project)
}

// ---- tool broker ------------------------------------------------------------

func (m *Manager) AuthorizeToolCall(sessionID, tool, action, resource string) (bool, string) {
	if action == "secret:read" {
		return false, "secret:read is not permitted for AI tool calls"
	}
	if strings.Contains(action, "gpu") {
		return false, "gpu actions are not permitted for AI tool calls"
	}
	return true, ""
}

func (m *Manager) RecordToolCall(sessionID, tool, action, resource, decision, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tc := ToolCall{
		ID:        newID("tc"),
		SessionID: sessionID,
		Tool:      tool,
		Action:    action,
		Resource:  resource,
		Decision:  decision,
		Reason:    reason,
		CalledAt:  now,
	}
	return m.store.InsertToolCall(tc)
}

func (m *Manager) ListToolCalls(sessionID string) ([]ToolCall, error) {
	return m.store.ListToolCalls(sessionID)
}

// ApprovalRequest represents a pending human-approval request for a high-risk action.
type ApprovalRequest struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	AgentID   string `json:"agentId"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Reason    string `json:"reason"`
	Status    string `json:"status"` // "pending", "approved", "denied"
	CreatedAt string `json:"createdAt"`
}

var highRiskActions = []string{
	"gpu:assign", "gpu:use", "instance:run",
	"network:create", "secret:read", "secret:create",
}

// RequiresApproval returns true if the action needs human approval.
func RequiresApproval(action string) bool {
	for _, a := range highRiskActions {
		if action == a || strings.HasPrefix(action, a+":") {
			return true
		}
	}
	return false
}

// BlocksResourceEscalation returns true if the action would escalate resources beyond
// what the agent was originally granted.
func (m *Manager) BlocksResourceEscalation(sessionID, action string) (bool, string) {
	escalationActions := []string{"gpu:assign", "type:create", "type:update"}
	for _, a := range escalationActions {
		if action == a {
			return true, fmt.Sprintf("action %q would escalate AI agent resource envelope", action)
		}
	}
	return false, ""
}

// ContentFirewall checks prompt/context for potential secret leakage patterns.
// Returns (safe bool, reason string).
func ContentFirewall(content string) (bool, string) {
	dangerous := []string{
		"BEGIN RSA PRIVATE KEY", "BEGIN EC PRIVATE KEY",
		"BEGIN OPENSSH PRIVATE KEY", "password=", "secret=",
		"api_key=", "token=", "Authorization: Bearer",
	}
	lower := strings.ToLower(content)
	for _, d := range dangerous {
		if strings.Contains(lower, strings.ToLower(d)) {
			return false, fmt.Sprintf("content firewall: potential secret detected (%s)", d)
		}
	}
	return true, ""
}

// ---- model gateway (Block 9 Ph4) --------------------------------------------

// AllowedModel represents a model registered in the gateway.
type AllowedModel struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	ModelID   string `json:"modelId"`   // e.g. "claude-opus-4-8"
	MaxTokens int    `json:"maxTokens"` // 0 = no limit
	CreatedAt string `json:"createdAt"`
}

var allowedModels []AllowedModel // in-memory registry; persistent path is out of scope

// RegisterModel adds a model to the gateway allowlist for a project.
func (m *Manager) RegisterModel(project, modelID string, maxTokens int) AllowedModel {
	am := AllowedModel{
		ID:        newID("model"),
		Project:   project,
		ModelID:   modelID,
		MaxTokens: maxTokens,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	allowedModels = append(allowedModels, am)
	return am
}

// ModelAllowed returns true when modelID is in the gateway allowlist for the project
// or when no models have been registered (open policy).
func (m *Manager) ModelAllowed(project, modelID string) bool {
	var projectModels []AllowedModel
	for _, am := range allowedModels {
		if am.Project == project {
			projectModels = append(projectModels, am)
		}
	}
	if len(projectModels) == 0 {
		return true // open policy
	}
	for _, am := range projectModels {
		if am.ModelID == modelID {
			return true
		}
	}
	return false
}

// ListAllowedModels returns the gateway allowlist for a project.
func (m *Manager) ListAllowedModels(project string) []AllowedModel {
	var out []AllowedModel
	for _, am := range allowedModels {
		if am.Project == project {
			out = append(out, am)
		}
	}
	return out
}

// ---- data governance (Block 9 Ph4) -----------------------------------------

// DataClass labels a data classification level.
type DataClass string

const (
	DataClassPublic       DataClass = "public"
	DataClassInternal     DataClass = "internal"
	DataClassConfidential DataClass = "confidential"
	DataClassSecret       DataClass = "secret"
)

// DataAccessPolicy governs what data classes an AI session may read.
type DataAccessPolicy struct {
	SessionID      string      `json:"sessionId"`
	AllowedClasses []DataClass `json:"allowedClasses"`
}

var dataAccessPolicies []DataAccessPolicy

// SetDataAccessPolicy records the allowed data classes for a session.
func (m *Manager) SetDataAccessPolicy(sessionID string, allowed []DataClass) {
	for i, p := range dataAccessPolicies {
		if p.SessionID == sessionID {
			dataAccessPolicies[i].AllowedClasses = allowed
			return
		}
	}
	dataAccessPolicies = append(dataAccessPolicies, DataAccessPolicy{
		SessionID:      sessionID,
		AllowedClasses: allowed,
	})
}

// DataAccessAllowed returns true when the session is permitted to read the given class.
func (m *Manager) DataAccessAllowed(sessionID string, class DataClass) bool {
	for _, p := range dataAccessPolicies {
		if p.SessionID != sessionID {
			continue
		}
		for _, c := range p.AllowedClasses {
			if c == class {
				return true
			}
		}
		return false
	}
	return true // no policy → open
}

// ---- agent memory governance (Block 9 Ph4) ----------------------------------

// MemoryScope defines the visibility of an agent memory store.
type MemoryScope string

const (
	MemoryScopeSession MemoryScope = "session" // cleared when session ends
	MemoryScopeAgent   MemoryScope = "agent"   // persists across sessions for an agent
	MemoryScopeProject MemoryScope = "project" // shared across all agents in a project
)

// MemoryEntry is a key-value record in an agent's memory store.
type MemoryEntry struct {
	Key       string      `json:"key"`
	Value     string      `json:"value"`
	Scope     MemoryScope `json:"scope"`
	AgentID   string      `json:"agentId"`
	ExpiresAt string      `json:"expiresAt,omitempty"` // RFC3339; empty = no TTL
	CreatedAt string      `json:"createdAt"`
}

var agentMemory []MemoryEntry

// WriteMemory stores a key-value entry scoped to an agent/session.
func (m *Manager) WriteMemory(agentID, key, value string, scope MemoryScope, ttl time.Duration) MemoryEntry {
	now := time.Now().UTC()
	exp := ""
	if ttl > 0 {
		exp = now.Add(ttl).Format(time.RFC3339)
	}
	entry := MemoryEntry{
		Key:       key,
		Value:     value,
		Scope:     scope,
		AgentID:   agentID,
		ExpiresAt: exp,
		CreatedAt: now.Format(time.RFC3339),
	}
	for i, e := range agentMemory {
		if e.AgentID == agentID && e.Key == key && e.Scope == scope {
			agentMemory[i] = entry
			return entry
		}
	}
	agentMemory = append(agentMemory, entry)
	return entry
}

// ReadMemory returns the value for the given key, agent, and scope.
// Returns ("", false) when the key does not exist or has expired.
func (m *Manager) ReadMemory(agentID, key string, scope MemoryScope) (string, bool) {
	now := time.Now().UTC()
	for _, e := range agentMemory {
		if e.AgentID != agentID || e.Key != key || e.Scope != scope {
			continue
		}
		if e.ExpiresAt != "" {
			exp, err := time.Parse(time.RFC3339, e.ExpiresAt)
			if err == nil && now.After(exp) {
				return "", false
			}
		}
		return e.Value, true
	}
	return "", false
}

// PurgeSessionMemory removes all session-scoped memory entries for an agent.
func (m *Manager) PurgeSessionMemory(agentID string) {
	filtered := agentMemory[:0]
	for _, e := range agentMemory {
		if !(e.AgentID == agentID && e.Scope == MemoryScopeSession) {
			filtered = append(filtered, e)
		}
	}
	agentMemory = filtered
}

// ---- AI resource risk scoring (Block 9 Ph4) --------------------------------

// RiskLevel classifies the risk of an agent action.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// RiskEvent records a scored action for audit/alerting.
type RiskEvent struct {
	SessionID string    `json:"sessionId"`
	AgentID   string    `json:"agentId"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Level     RiskLevel `json:"level"`
	Score     int       `json:"score"` // 0-100
	Reason    string    `json:"reason"`
	Timestamp string    `json:"timestamp"`
}

// ScoreAction assigns a risk level and score to an agent action.
func ScoreAction(action, resource string) RiskEvent {
	score := 10
	level := RiskLow
	reason := "routine read operation"

	switch {
	case strings.HasPrefix(action, "secret:") || strings.HasPrefix(action, "kms:"):
		score, level, reason = 90, RiskCritical, "secret/key material access"
	case strings.HasPrefix(action, "gpu:") || action == "instance:run":
		score, level, reason = 75, RiskHigh, "resource allocation that may incur significant cost"
	case strings.HasPrefix(action, "network:") || strings.HasPrefix(action, "firewall:"):
		score, level, reason = 60, RiskHigh, "network topology change"
	case strings.HasPrefix(action, "iam:") || strings.HasPrefix(action, "policy:"):
		score, level, reason = 70, RiskHigh, "permission or policy mutation"
	case strings.HasSuffix(action, ":delete") || strings.HasSuffix(action, ":destroy"):
		score, level, reason = 55, RiskMedium, "irreversible delete operation"
	case strings.HasSuffix(action, ":create") || strings.HasSuffix(action, ":update"):
		score, level, reason = 30, RiskMedium, "mutating write operation"
	}

	return RiskEvent{
		Action:    action,
		Resource:  resource,
		Level:     level,
		Score:     score,
		Reason:    reason,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
