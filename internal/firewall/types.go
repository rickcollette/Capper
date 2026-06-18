package firewall

// Firewall modes
const (
	ModeStrict     = "strict"
	ModePermissive = "permissive"
	ModeInternal   = "internal"

	ActionAllow  = "allow"
	ActionDeny   = "deny"
	ActionReject = "reject"

	DirectionIngress = "ingress"
	DirectionEgress  = "egress"
	DirectionForward = "forward"
	DirectionAny     = "any"

	EndpointAny      = "any"
	EndpointInternet = "internet"
	EndpointHost     = "host"
	EndpointGateway  = "gateway"
	EndpointNetwork  = "network"
	EndpointInstance = "instance"
	EndpointLabel    = "label"
	EndpointCIDR     = "cidr"

	StatusPending = "pending"
	StatusApplied = "applied"
	StatusError   = "error"
)

// Firewall holds the policy configuration for a single network.
type Firewall struct {
	NetworkID            string `json:"networkID"`
	NetworkName          string `json:"networkName"`
	Mode                 string `json:"mode"`
	Backend              string `json:"backend"`
	DefaultForwardPolicy string `json:"defaultForwardPolicy"`
	DefaultIngressPolicy string `json:"defaultIngressPolicy"`
	DefaultEgressPolicy  string `json:"defaultEgressPolicy"`
	AllowDNS             bool   `json:"allowDNS"`
	AllowEstablished     bool   `json:"allowEstablished"`
	NATEnabled           bool   `json:"natEnabled"`
	Status               string `json:"status"`
	LastAppliedAt        string `json:"lastAppliedAt"`
}

// Endpoint describes one side (source or destination) of a firewall rule.
type Endpoint struct {
	Type  string `json:"type"`
	Key   string `json:"key,omitempty"`   // label key
	Value string `json:"value,omitempty"` // label value, instance ID, CIDR
}

// Rule is a single firewall policy entry.
type Rule struct {
	ID          string   `json:"id"`
	NetworkID   string   `json:"networkID"`
	Priority    int      `json:"priority"`
	Enabled     bool     `json:"enabled"`
	Action      string   `json:"action"`
	Direction   string   `json:"direction"`
	From        Endpoint `json:"from"`
	To          Endpoint `json:"to"`
	Protocol    string   `json:"protocol"`
	Ports       []int    `json:"ports"`
	Description string   `json:"description"`
	CreatedAt   string   `json:"createdAt"`
}

// RuleSpec carries user-supplied fields for creating a rule.
type RuleSpec struct {
	Priority    int
	Action      string
	Direction   string
	From        Endpoint
	To          Endpoint
	Protocol    string
	Ports       []int
	Description string
}

// NetworkInfo is the firewall-visible snapshot of a network.
// It has no dependency on the network package, avoiding import cycles.
type NetworkInfo struct {
	ID      string
	Name    string
	Subnet  string
	Gateway string
	Bridge  string
	Mode    string // network mode (nat/isolated/etc.)
}

// ApplyResult is returned by Apply/dry-run operations.
type ApplyResult struct {
	DryRun  bool     `json:"dryRun"`
	Script  string   `json:"script"`
	Applied bool     `json:"applied"`
	Error   string   `json:"error,omitempty"`
}
