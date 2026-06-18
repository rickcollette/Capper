package lb

// LBMode is the load balancing protocol.
type LBMode string

const (
	ModeTCP  LBMode = "tcp"
	ModeHTTP LBMode = "http"
)

// LBStatus tracks whether the load balancer is active.
type LBStatus string

const (
	StatusActive  LBStatus = "active"
	StatusStopped LBStatus = "stopped"
)

// LBAlgorithm controls how backends are selected.
type LBAlgorithm string

const (
	AlgoRoundRobin       LBAlgorithm = "round-robin"
	AlgoLeastConnections LBAlgorithm = "least-connections"
)

// LoadBalancer is a virtual TCP/HTTP load balancer attached to a network.
type LoadBalancer struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Project      string      `json:"project"`
	NetworkID    string      `json:"networkId,omitempty"`
	Mode         LBMode      `json:"mode"`
	ListenAddr   string      `json:"listenAddr"` // e.g. "0.0.0.0:8080"
	Status       LBStatus    `json:"status"`
	Algorithm    LBAlgorithm `json:"algorithm,omitempty"` // default: round-robin
	Selector     string      `json:"selector,omitempty"`  // "key=val" label selector
	TLSCertName  string      `json:"tlsCertName,omitempty"`
	ServiceAlias string      `json:"serviceAlias,omitempty"` // optional DNS CNAME alias
	CreatedAt    string      `json:"createdAt"`
}

// Backend is an upstream endpoint served by a load balancer.
type Backend struct {
	ID      string `json:"id"`
	LBID    string `json:"lbId"`
	Address string `json:"address"` // "ip:port"
	Healthy bool   `json:"healthy"`
}
