package cappersdk

import (
	"context"
	"fmt"
)

// ---- shared topology types --------------------------------------------------

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

type Node struct {
	ID            string            `json:"id"`
	RealmID       string            `json:"realmId"`
	RegionID      string            `json:"regionId"`
	ZoneID        string            `json:"zoneId"`
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Status        string            `json:"status"`
	FailureDomain string            `json:"failureDomain"`
	Labels        map[string]string `json:"labels,omitempty"`
	CPUCount      int               `json:"cpuCount"`
	MemoryBytes   int64             `json:"memoryBytes"`
	DiskBytes     int64             `json:"diskBytes"`
	CreatedAt     string            `json:"createdAt"`
	UpdatedAt     string            `json:"updatedAt"`
}

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

type PlacementRequest struct {
	Project      string            `json:"project"`
	Image        string            `json:"image"`
	InstanceType string            `json:"instanceType"`
	CPURequired  int               `json:"cpuRequired"`
	MemoryBytes  int64             `json:"memoryBytes"`
	GPURequired  bool              `json:"gpuRequired"`
	RequireLabel map[string]string `json:"requireLabel,omitempty"`
	Region       string            `json:"region"`
	Zone         string            `json:"zone"`
	Strategy     string            `json:"strategy"`
	MinZones     int               `json:"minZones"`
	AntiAffinity map[string]string `json:"antiAffinity,omitempty"`
}

type PlacementCandidate struct {
	RegionID string   `json:"regionId"`
	ZoneID   string   `json:"zoneId"`
	NodeID   string   `json:"nodeId"`
	Region   string   `json:"region"`
	Zone     string   `json:"zone"`
	Node     string   `json:"node"`
	Score    int      `json:"score"`
	Reasons  []string `json:"reasons"`
}

type PlacementRejection struct {
	NodeID string `json:"nodeId"`
	Node   string `json:"node"`
	Reason string `json:"reason"`
}

type PlacementResult struct {
	Allowed    bool                 `json:"allowed"`
	Candidates []PlacementCandidate `json:"candidates"`
	Rejections []PlacementRejection `json:"rejections"`
}

// ---- RealmsAPI --------------------------------------------------------------

type RealmsAPI struct{ c *Client }

func (a *RealmsAPI) List(ctx context.Context) ([]Realm, error) {
	var env struct{ Data []Realm }
	return env.Data, a.c.get(ctx, "realms", &env)
}

func (a *RealmsAPI) Create(ctx context.Context, r Realm) (Realm, error) {
	var env struct{ Data Realm }
	return env.Data, a.c.post(ctx, "realms", r, &env)
}

func (a *RealmsAPI) Get(ctx context.Context, slugOrID string) (Realm, error) {
	var env struct{ Data Realm }
	return env.Data, a.c.get(ctx, "realms/"+slugOrID, &env)
}

func (a *RealmsAPI) Delete(ctx context.Context, slugOrID string) error {
	return a.c.del(ctx, "realms/"+slugOrID)
}

// ---- RegionsAPI -------------------------------------------------------------

type RegionsAPI struct{ c *Client }

func (a *RegionsAPI) List(ctx context.Context) ([]Region, error) {
	var env struct{ Data []Region }
	return env.Data, a.c.get(ctx, "regions", &env)
}

func (a *RegionsAPI) ListByRealm(ctx context.Context, realm string) ([]Region, error) {
	var env struct{ Data []Region }
	return env.Data, a.c.get(ctx, "regions?realm="+realm, &env)
}

func (a *RegionsAPI) Create(ctx context.Context, r Region) (Region, error) {
	var env struct{ Data Region }
	return env.Data, a.c.post(ctx, "regions", r, &env)
}

func (a *RegionsAPI) Get(ctx context.Context, slugOrID string) (Region, error) {
	var env struct{ Data Region }
	return env.Data, a.c.get(ctx, "regions/"+slugOrID, &env)
}

func (a *RegionsAPI) Delete(ctx context.Context, slugOrID string) error {
	return a.c.del(ctx, "regions/"+slugOrID)
}

func (a *RegionsAPI) Drain(ctx context.Context, slugOrID string) (Region, error) {
	var env struct{ Data Region }
	return env.Data, a.c.post(ctx, "regions/"+slugOrID+"/drain", nil, &env)
}

func (a *RegionsAPI) Evacuate(ctx context.Context, slugOrID string) (Region, error) {
	var env struct{ Data Region }
	return env.Data, a.c.post(ctx, "regions/"+slugOrID+"/evacuate", nil, &env)
}

// ---- ZonesAPI ---------------------------------------------------------------

type ZonesAPI struct{ c *Client }

func (a *ZonesAPI) List(ctx context.Context) ([]Zone, error) {
	var env struct{ Data []Zone }
	return env.Data, a.c.get(ctx, "zones", &env)
}

func (a *ZonesAPI) ListByRegion(ctx context.Context, region string) ([]Zone, error) {
	var env struct{ Data []Zone }
	return env.Data, a.c.get(ctx, "zones?region="+region, &env)
}

func (a *ZonesAPI) Create(ctx context.Context, z Zone) (Zone, error) {
	var env struct{ Data Zone }
	return env.Data, a.c.post(ctx, "zones", z, &env)
}

func (a *ZonesAPI) Get(ctx context.Context, slugOrID string) (Zone, error) {
	var env struct{ Data Zone }
	return env.Data, a.c.get(ctx, "zones/"+slugOrID, &env)
}

func (a *ZonesAPI) Delete(ctx context.Context, slugOrID string) error {
	return a.c.del(ctx, "zones/"+slugOrID)
}

func (a *ZonesAPI) Cordon(ctx context.Context, slugOrID string) (Zone, error) {
	var env struct{ Data Zone }
	return env.Data, a.c.post(ctx, "zones/"+slugOrID+"/cordon", nil, &env)
}

func (a *ZonesAPI) Uncordon(ctx context.Context, slugOrID string) (Zone, error) {
	var env struct{ Data Zone }
	return env.Data, a.c.post(ctx, "zones/"+slugOrID+"/uncordon", nil, &env)
}

func (a *ZonesAPI) Drain(ctx context.Context, slugOrID string) (Zone, error) {
	var env struct{ Data Zone }
	return env.Data, a.c.post(ctx, "zones/"+slugOrID+"/drain", nil, &env)
}

// ---- NodesAPI ---------------------------------------------------------------

type NodesAPI struct{ c *Client }

func (a *NodesAPI) List(ctx context.Context) ([]Node, error) {
	var env struct{ Data []Node }
	return env.Data, a.c.get(ctx, "nodes", &env)
}

func (a *NodesAPI) ListByZone(ctx context.Context, zone string) ([]Node, error) {
	var env struct{ Data []Node }
	return env.Data, a.c.get(ctx, "nodes?zone="+zone, &env)
}

func (a *NodesAPI) Register(ctx context.Context, n Node) (Node, error) {
	var env struct{ Data Node }
	return env.Data, a.c.post(ctx, "nodes", n, &env)
}

func (a *NodesAPI) Get(ctx context.Context, slugOrID string) (Node, error) {
	var env struct{ Data Node }
	return env.Data, a.c.get(ctx, "nodes/"+slugOrID, &env)
}

func (a *NodesAPI) Delete(ctx context.Context, slugOrID string) error {
	return a.c.del(ctx, "nodes/"+slugOrID)
}

func (a *NodesAPI) Cordon(ctx context.Context, slugOrID string) (Node, error) {
	var env struct{ Data Node }
	return env.Data, a.c.post(ctx, "nodes/"+slugOrID+"/cordon", nil, &env)
}

func (a *NodesAPI) Uncordon(ctx context.Context, slugOrID string) (Node, error) {
	var env struct{ Data Node }
	return env.Data, a.c.post(ctx, "nodes/"+slugOrID+"/uncordon", nil, &env)
}

func (a *NodesAPI) Drain(ctx context.Context, slugOrID string) (Node, error) {
	var env struct{ Data Node }
	return env.Data, a.c.post(ctx, "nodes/"+slugOrID+"/drain", nil, &env)
}

// ---- VPCsAPI ----------------------------------------------------------------

type VPCsAPI struct{ c *Client }

func (a *VPCsAPI) List(ctx context.Context, project string) ([]VPC, error) {
	var env struct{ Data []VPC }
	path := fmt.Sprintf("vpcs?project=%s", project)
	return env.Data, a.c.get(ctx, path, &env)
}

func (a *VPCsAPI) Create(ctx context.Context, v VPC) (VPC, error) {
	var env struct{ Data VPC }
	return env.Data, a.c.post(ctx, "vpcs", v, &env)
}

func (a *VPCsAPI) Get(ctx context.Context, slugOrID string) (VPC, error) {
	var env struct{ Data VPC }
	return env.Data, a.c.get(ctx, "vpcs/"+slugOrID, &env)
}

func (a *VPCsAPI) Delete(ctx context.Context, slugOrID string) error {
	return a.c.del(ctx, "vpcs/"+slugOrID)
}

// ---- SchedulerAPI -----------------------------------------------------------

type SchedulerAPI struct{ c *Client }

func (a *SchedulerAPI) Simulate(ctx context.Context, req PlacementRequest) (PlacementResult, error) {
	var env struct{ Data PlacementResult }
	return env.Data, a.c.post(ctx, "scheduler/simulate", req, &env)
}

func (a *SchedulerAPI) Capacity(ctx context.Context, region, zone string) (map[string]any, error) {
	path := "scheduler/capacity"
	if region != "" {
		path += "?region=" + region
	} else if zone != "" {
		path += "?zone=" + zone
	}
	var env struct{ Data map[string]any }
	return env.Data, a.c.get(ctx, path, &env)
}
