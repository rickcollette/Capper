// Package topology implements the Realm → Region → Zone → Node hierarchy.
package topology

import "errors"

// ErrNotFound is returned when a topology resource is not found.
var ErrNotFound = errors.New("topology: not found")

// ---- Realm ------------------------------------------------------------------

type Realm struct {
	ID          string            `json:"id"`
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// ---- Region -----------------------------------------------------------------

type Region struct {
	ID          string            `json:"id"`
	RealmID     string            `json:"realmId"`
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Location    string            `json:"location"`
	Country     string            `json:"country"`
	RegionCode  string            `json:"regionCode"`
	Latitude    float64           `json:"latitude"`
	Longitude   float64           `json:"longitude"`
	Status      string            `json:"status"`
	ControlURL  string            `json:"controlUrl"`
	APIURL      string            `json:"apiUrl"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// ---- Zone -------------------------------------------------------------------

type Zone struct {
	ID            string            `json:"id"`
	RealmID       string            `json:"realmId"`
	RegionID      string            `json:"regionId"`
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	FailureDomain string            `json:"failureDomain"`
	Status        string            `json:"status"`
	ControlURL    string            `json:"controlUrl"`
	NetworkCIDR   string            `json:"networkCidr"`
	Labels        map[string]string `json:"labels,omitempty"`
	CreatedAt     string            `json:"createdAt"`
	UpdatedAt     string            `json:"updatedAt"`
}

// ---- Node -------------------------------------------------------------------

type Node struct {
	ID             string            `json:"id"`
	RealmID        string            `json:"realmId"`
	RegionID       string            `json:"regionId"`
	ZoneID         string            `json:"zoneId"`
	Slug           string            `json:"slug"`
	Name           string            `json:"name"`
	Address        string            `json:"address"`
	Status         string            `json:"status"`
	FailureDomain  string            `json:"failureDomain"`
	Labels         map[string]string `json:"labels,omitempty"`
	CPUCount       int               `json:"cpuCount"`
	MemoryBytes    int64             `json:"memoryBytes"`
	DiskBytes      int64             `json:"diskBytes"`
	Roles          []string          `json:"roles,omitempty"`
	Taints         []NodeTaint       `json:"taints,omitempty"`
	Cordoned       bool              `json:"cordoned"`
	AgentVersion   string            `json:"agentVersion"`
	LastHeartbeat  string            `json:"lastHeartbeat"`
	GPUCount       int               `json:"gpuCount"`
	GPUMemoryBytes int64             `json:"gpuMemoryBytes"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
}

// NodeTaint is a scheduling taint on a node.
type NodeTaint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"` // "NoSchedule", "PreferNoSchedule", "NoExecute"
}

// NodeService tracks a service running on a node.
type NodeService struct {
	NodeID       string `json:"nodeId"`
	ServiceName  string `json:"serviceName"`
	DesiredState string `json:"desiredState"`
	ActualState  string `json:"actualState"`
	Version      string `json:"version"`
	Health       string `json:"health"`
	Message      string `json:"message"`
	LastSeen     string `json:"lastSeen"`
}

// NodeHeartbeat is the most recent heartbeat from a node agent.
type NodeHeartbeat struct {
	NodeID          string `json:"nodeId"`
	Status          string `json:"status"`
	CPUUsed         int    `json:"cpuUsed"`
	MemoryUsedBytes int64  `json:"memoryUsedBytes"`
	DiskUsedBytes   int64  `json:"diskUsedBytes"`
	GPUUsed         int    `json:"gpuUsed"`
	Message         string `json:"message"`
	SeenAt          string `json:"seenAt"`
	// Version is the node agent's build version, reported each heartbeat so the
	// control plane can track per-node version skew during rolling upgrades.
	Version string `json:"version,omitempty"`
}

// NodePool groups nodes for scheduling and lifecycle management.
type NodePool struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	RealmID         string            `json:"realmId"`
	RegionID        string            `json:"regionId"`
	ZoneID          string            `json:"zoneId"`
	Roles           []string          `json:"roles"`
	Labels          map[string]string `json:"labels,omitempty"`
	MinNodes        int               `json:"minNodes"`
	DesiredNodes    int               `json:"desiredNodes"`
	MaxNodes        int               `json:"maxNodes"`
	PlacementPolicy string            `json:"placementPolicy"`
	Status          string            `json:"status"`
	MemberCount     int               `json:"memberCount"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
}

// JoinToken allows a new node to self-register.
type JoinToken struct {
	ID        string   `json:"id"`
	Token     string   `json:"token"`
	RealmID   string   `json:"realmId"`
	RegionID  string   `json:"regionId"`
	ZoneID    string   `json:"zoneId"`
	Roles     []string `json:"roles"`
	UsesLeft  int      `json:"usesLeft"`
	ExpiresAt string   `json:"expiresAt"`
	CreatedBy string   `json:"createdBy"`
	CreatedAt string   `json:"createdAt"`
}

// NodeToleration allows a workload to be scheduled on a tainted node.
type NodeToleration struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// ---- VPC --------------------------------------------------------------------

type VPC struct {
	ID             string            `json:"id"`
	RealmID        string            `json:"realmId"`
	Project        string            `json:"project"`
	Slug           string            `json:"slug"`
	Name           string            `json:"name"`
	CIDR           string            `json:"cidr"`
	Status         string            `json:"status"`
	HomeRegionID   string            `json:"homeRegionId"`
	MobilityPolicy string            `json:"mobilityPolicy"`
	Labels         map[string]string `json:"labels,omitempty"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
}

// ---- VPCSubnet --------------------------------------------------------------

type VPCSubnet struct {
	ID        string `json:"id"`
	VPCID     string `json:"vpcId"`
	RealmID   string `json:"realmId"`
	RegionID  string `json:"regionId"`
	ZoneID    string `json:"zoneId"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	Gateway   string `json:"gateway"`
	Bridge    string `json:"bridge"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ---- VPCRoute ---------------------------------------------------------------

type VPCRoute struct {
	ID         string `json:"id"`
	VPCID      string `json:"vpcId"`
	Scope      string `json:"scope"`
	RealmID    string `json:"realmId"`
	RegionID   string `json:"regionId"`
	ZoneID     string `json:"zoneId"`
	DestCIDR   string `json:"destCidr"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Priority   int    `json:"priority"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// ---- PlacementPolicy --------------------------------------------------------

type PlacementPolicy struct {
	ID               string            `json:"id"`
	RealmID          string            `json:"realmId"`
	Project          string            `json:"project"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Scope            string            `json:"scope"`
	Strategy         string            `json:"strategy"`
	MinRegions       int               `json:"minRegions"`
	MinZones         int               `json:"minZones"`
	MaxZones         int               `json:"maxZones"`
	PreferredRegions []string          `json:"preferredRegions,omitempty"`
	PreferredZones   []string          `json:"preferredZones,omitempty"`
	RequiredLabels   map[string]string `json:"requiredLabels,omitempty"`
	AntiAffinity     map[string]string `json:"antiAffinity,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
}

// ---- ImageReplica -----------------------------------------------------------

type ImageReplica struct {
	ID        string `json:"id"`
	ImageID   string `json:"imageId"`
	Digest    string `json:"digest"`
	RealmID   string `json:"realmId"`
	RegionID  string `json:"regionId"`
	ZoneID    string `json:"zoneId"`
	NodeID    string `json:"nodeId"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"sizeBytes"`
	Location  string `json:"location"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ---- StorageReplica ---------------------------------------------------------

type StorageReplica struct {
	ID        string `json:"id"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"objectKey"`
	ETag      string `json:"etag"`
	RealmID   string `json:"realmId"`
	RegionID  string `json:"regionId"`
	ZoneID    string `json:"zoneId"`
	NodeID    string `json:"nodeId"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"sizeBytes"`
	Location  string `json:"location"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ---- ServiceHealth ----------------------------------------------------------

type ServiceHealth struct {
	ID          string `json:"id"`
	Scope       string `json:"scope"`
	RealmID     string `json:"realmId"`
	RegionID    string `json:"regionId"`
	ZoneID      string `json:"zoneId"`
	NodeID      string `json:"nodeId"`
	ServiceName string `json:"serviceName"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	CheckedAt   string `json:"checkedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// ---- MigrationPlan ----------------------------------------------------------

type MigrationPlan struct {
	ID             string `json:"id"`
	RealmID        string `json:"realmId"`
	Project        string `json:"project"`
	Name           string `json:"name"`
	MigrationType  string `json:"migrationType"`
	SourceRegionID string `json:"sourceRegionId"`
	TargetRegionID string `json:"targetRegionId"`
	SourceZoneID   string `json:"sourceZoneId"`
	TargetZoneID   string `json:"targetZoneId"`
	VPCID          string `json:"vpcId"`
	Status         string `json:"status"`
	PlanJSON       string `json:"planJson,omitempty"`
	ResultJSON     string `json:"resultJson,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// ---- Scheduler types --------------------------------------------------------

// PlacementRequest is the input to the scheduler.
type PlacementRequest struct {
	Project        string            `json:"project"`
	Image          string            `json:"image"`
	InstanceType   string            `json:"instanceType"`
	CPURequired    int               `json:"cpuRequired"`
	MemoryBytes    int64             `json:"memoryBytes"`
	DiskBytes      int64             `json:"diskBytes,omitempty"`
	GPURequired    bool              `json:"gpuRequired"`
	GPUCount       int               `json:"gpuCount,omitempty"`
	RequireLabel   map[string]string `json:"requireLabel,omitempty"`
	Region         string            `json:"region"`
	Zone           string            `json:"zone"`
	Strategy       string            `json:"strategy"`
	MinZones       int               `json:"minZones"`
	AntiAffinity   map[string]string `json:"antiAffinity,omitempty"`
	RequiredRoles  []string          `json:"requiredRoles,omitempty"`
	PreferredRoles []string          `json:"preferredRoles,omitempty"`
	Tolerations    []NodeToleration  `json:"tolerations,omitempty"`
}

// Candidate is a scored placement candidate.
type Candidate struct {
	RealmID  string   `json:"realmId"`
	RegionID string   `json:"regionId"`
	ZoneID   string   `json:"zoneId"`
	NodeID   string   `json:"nodeId"`
	Region   string   `json:"region"`
	Zone     string   `json:"zone"`
	Node     string   `json:"node"`
	Score    int      `json:"score"`
	Reasons  []string `json:"reasons"`
}

// Rejection explains why a node was rejected.
type Rejection struct {
	NodeID string `json:"nodeId"`
	Node   string `json:"node"`
	Reason string `json:"reason"`
}

// PlacementResult is the scheduler's response.
type PlacementResult struct {
	Allowed    bool        `json:"allowed"`
	Candidates []Candidate `json:"candidates"`
	Rejections []Rejection `json:"rejections"`
}

// ---- Status / strategy constants --------------------------------------------

const (
	StatusActive      = "active"
	StatusDisabled    = "disabled"
	StatusMaintenance = "maintenance"
	StatusDeleting    = "deleting"
	StatusDraining    = "draining"
	StatusUnhealthy   = "unhealthy"
	StatusDegraded    = "degraded"
	StatusCordoned    = "cordoned"
	StatusDrained     = "drained"
	StatusReady       = "ready"
	StatusOffline     = "offline"
	StatusLost        = "lost"

	StrategySpreadZones   = "spread-zones"
	StrategySpreadRegions = "spread-regions"
	StrategySingleZone    = "single-zone"
	StrategySingleNode    = "single-node"
	StrategyPack          = "pack"
	StrategyManual        = "manual"

	MobilityManual        = "manual"
	MobilityRegionMovable = "region-movable"
	MobilityRealmDR       = "realm-dr"
	MobilityActiveActive  = "active-active"
	MobilityLocked        = "locked"
)
