// Package ipam implements Capper's Public IPAM — routable IP pools and
// reservations, Capper's equivalent of AWS Elastic IPs. Pools are CIDR ranges
// owned by the platform; individual addresses are reserved, attached, and bound
// to load balancers, VPC egress NAT, or passthrough hosts. The control plane
// decides ownership; network-role node agents apply it.
package ipam

// RoutableIPPool is a configured CIDR block that addresses are allocated from.
type RoutableIPPool struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	CIDR              string   `json:"cidr"`
	IPVersion         int      `json:"ipVersion"`
	Scope             string   `json:"scope"` // global, realm, region, zone, node
	RealmID           string   `json:"realmId,omitempty"`
	RegionID          string   `json:"regionId,omitempty"`
	ZoneID            string   `json:"zoneId,omitempty"`
	NodeID            string   `json:"nodeId,omitempty"`
	Gateway           string   `json:"gateway,omitempty"`
	VLANID            int      `json:"vlanId,omitempty"`
	InterfaceName     string   `json:"interfaceName,omitempty"`
	Usage             []string `json:"usage"`
	Status            string   `json:"status"`
	AllowAutoAllocate bool     `json:"allowAutoAllocate"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

// RoutableIP is a single address inside a pool.
type RoutableIP struct {
	ID             string `json:"id"`
	PoolID         string `json:"poolId"`
	Address        string `json:"address"`
	Status         string `json:"status"`
	Project        string `json:"project,omitempty"`
	Name           string `json:"name,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	AllocationType string `json:"allocationType"` // auto, reserved, system
	TargetType     string `json:"targetType,omitempty"`
	TargetID       string `json:"targetId,omitempty"`
	DNSName        string `json:"dnsName,omitempty"`
	Description    string `json:"description,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// IPBinding records an active attachment of an IP to a target.
type IPBinding struct {
	ID           string `json:"id"`
	IPID         string `json:"ipId"`
	TargetType   string `json:"targetType"`  // load-balancer, vpc-nat, instance, ingress, s3
	TargetID     string `json:"targetId"`
	BindingMode  string `json:"bindingMode"` // vip, snat, dnat, passthrough, floating
	Protocol     string `json:"protocol,omitempty"`
	ExternalPort int    `json:"externalPort,omitempty"`
	InternalIP   string `json:"internalIp,omitempty"`
	InternalPort int    `json:"internalPort,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// IP status values.
const (
	IPAvailable   = "available"
	IPReserved    = "reserved"
	IPAllocated   = "allocated"
	IPAttached    = "attached"
	IPDetaching   = "detaching"
	IPQuarantined = "quarantined"
	IPBlocked     = "blocked"
	IPRetired     = "retired"
	IPSystem      = "system"
)

// Pool status values.
const (
	PoolActive    = "active"
	PoolDraining  = "draining"
	PoolDisabled  = "disabled"
	PoolExhausted = "exhausted"
	PoolRetired   = "retired"
)

// Usage classes a pool may declare.
const (
	UsageIngress      = "ingress"
	UsageEgress       = "egress"
	UsageLoadBalancer = "load-balancer"
	UsageReserved     = "reserved"
	UsagePassthrough  = "passthrough"
	UsageSystem       = "system"
	UsageFloating     = "floating"
)

// Binding modes.
const (
	ModeVIP         = "vip"
	ModeSNAT        = "snat"
	ModeDNAT        = "dnat"
	ModePassthrough = "passthrough"
	ModeFloating    = "floating"
)
