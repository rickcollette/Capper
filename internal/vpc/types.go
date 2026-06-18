package vpc

// VPC is a logical network boundary within a project.
type VPC struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	DNSDomain string `json:"dnsDomain,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// SubnetKind controls whether traffic can route to the public internet.
type SubnetKind string

const (
	SubnetPublic   SubnetKind = "public"
	SubnetPrivate  SubnetKind = "private"
	SubnetIsolated SubnetKind = "isolated"
)

// Subnet is an address segment inside a VPC, backed by a capsw bridge.
type Subnet struct {
	ID         string     `json:"id"`
	VPCID      string     `json:"vpcId"`
	Name       string     `json:"name"`
	CIDR       string     `json:"cidr"`
	Zone       string     `json:"zone,omitempty"`
	Kind       SubnetKind `json:"kind"`
	BridgeName string     `json:"bridgeName"`
	GatewayIP  string     `json:"gatewayIp"`
	CreatedAt  string     `json:"createdAt"`
}

// RouteTable holds routing rules for one or more subnets.
type RouteTable struct {
	ID        string `json:"id"`
	VPCID     string `json:"vpcId"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Route is a single routing entry in a RouteTable.
type Route struct {
	ID             string `json:"id"`
	RouteTableID   string `json:"routeTableId"`
	DestinationCIDR string `json:"destinationCidr"`
	TargetType     string `json:"targetType"` // "igw", "nat", "local", "instance"
	TargetID       string `json:"targetId"`
}

// SecurityGroup is an instance-level firewall within a VPC.
type SecurityGroup struct {
	ID          string `json:"id"`
	VPCID       string `json:"vpcId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DefaultDeny bool   `json:"defaultDeny"`
	CreatedAt   string `json:"createdAt"`
}

// SGRuleDirection specifies whether a rule applies to inbound or outbound traffic.
type SGRuleDirection string

const (
	SGIngress SGRuleDirection = "ingress"
	SGEgress  SGRuleDirection = "egress"
)

// SecurityGroupRule is a single allow/deny rule within a SecurityGroup.
type SecurityGroupRule struct {
	ID              string          `json:"id"`
	SecurityGroupID string          `json:"securityGroupId"`
	Direction       SGRuleDirection `json:"direction"`
	Protocol        string          `json:"protocol"` // "tcp", "udp", "icmp", "all"
	FromPort        int             `json:"fromPort,omitempty"`
	ToPort          int             `json:"toPort,omitempty"`
	CIDR            string          `json:"cidr,omitempty"`
	SourceSGID      string          `json:"sourceSgId,omitempty"`
	Action          string          `json:"action"` // "allow", "deny"
}

// InternetGateway provides public internet access for a VPC.
type InternetGateway struct {
	ID        string `json:"id"`
	VPCID     string `json:"vpcId"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// NATGateway provides outbound internet access for private subnets.
type NATGateway struct {
	ID        string `json:"id"`
	VPCID     string `json:"vpcId"`
	SubnetID  string `json:"subnetId"`
	Name      string `json:"name"`
	PublicIP  string `json:"publicIp,omitempty"`
	CreatedAt string `json:"createdAt"`
}
