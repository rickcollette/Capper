package metadata

// InstanceMetadata is the per-instance metadata record stored at launch time.
type InstanceMetadata struct {
	InstanceID   string            `json:"instanceId"`
	Hostname     string            `json:"hostname"`
	Project      string            `json:"project"`
	Labels       map[string]string `json:"labels,omitempty"`
	InstanceType string            `json:"instanceType,omitempty"`
	NetworkIP    string            `json:"networkIp,omitempty"`
	Gateway      string            `json:"gateway,omitempty"`
	DNS          string            `json:"dns,omitempty"`
	UserData     string            `json:"userData,omitempty"`
	TokenHash    string            `json:"tokenHash,omitempty"` // SHA-256 of issued token
	CreatedAt    string            `json:"createdAt"`

	// AI capsule fields — populated only for AI instances.
	AISessionID      string   `json:"aiSessionId,omitempty"`
	AIAgentID        string   `json:"aiAgentId,omitempty"`
	AIModel          string   `json:"aiModel,omitempty"`
	AIAssumedRole    string   `json:"aiAssumedRole,omitempty"`
	AIMCPServers     []string `json:"aiMcpServers,omitempty"`
	AIToolBroker     string   `json:"aiToolBroker,omitempty"`
	AIModelGateway   string   `json:"aiModelGateway,omitempty"`
	AIApprovalEndpoint string `json:"aiApprovalEndpoint,omitempty"`
	// Policy fields.
	AllowedActions []string `json:"allowedActions,omitempty"`
	DeniedActions  []string `json:"deniedActions,omitempty"`
	ResourceLock   bool     `json:"resourceLock,omitempty"`
	// Secret references (never raw values).
	SecretRefs []SecretRef `json:"secretRefs,omitempty"`
}

// SecretRef is a reference to a secret (name + allowed use), never the raw value.
type SecretRef struct {
	Name       string `json:"name"`
	EnvVar     string `json:"envVar,omitempty"`
	AllowedUse string `json:"allowedUse,omitempty"` // e.g. "db-password", "api-key"
}

// AccessLog records a single metadata request for audit.
type AccessLog struct {
	ID         string `json:"id"`
	InstanceID string `json:"instanceId"`
	SourceIP   string `json:"sourceIp"`
	Endpoint   string `json:"endpoint"`
	Allowed    bool   `json:"allowed"`
	AuthStatus string `json:"authStatus"` // "public", "token_ok", "token_missing", "token_invalid"
	CreatedAt  string `json:"createdAt"`
}

// NetworkData is the network-data document served at /capper/v1/network-data.
type NetworkData struct {
	Interfaces []InterfaceInfo `json:"interfaces"`
	DNS        string          `json:"dns"`
	Gateway    string          `json:"gateway"`
}

// InterfaceInfo describes a network interface inside the instance.
type InterfaceInfo struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Gateway string `json:"gateway"`
}
