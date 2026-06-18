package cappersdk

import "context"

// ---- AI --------------------------------------------------------------------

// AIAPI accesses the AI control plane (agents, sessions, MCP registrations).
type AIAPI struct{ c *Client }

// AIAgent is a registered AI agent.
type AIAgent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Model  string `json:"model"`
	Status string `json:"status"`
}

// ListAgents returns registered AI agents.
func (a *AIAPI) ListAgents(ctx context.Context) ([]AIAgent, error) {
	var out struct {
		Data []AIAgent `json:"data"`
	}
	return out.Data, a.c.get(ctx, "ai/agents", &out)
}

// ListSessions returns AI sessions.
func (a *AIAPI) ListSessions(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		Data []map[string]any `json:"data"`
	}
	return out.Data, a.c.get(ctx, "ai/sessions", &out)
}

// ---- Autoscale / Placement policies ----------------------------------------

// AutoscaleAPI accesses autoscale policies.
type AutoscaleAPI struct{ c *Client }

// ScalePolicy is an autoscale policy.
type ScalePolicy struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GroupID    string `json:"groupId"`
	GroupName  string `json:"groupName,omitempty"`
	Enabled    bool   `json:"enabled"`
	PolicyType string `json:"policyType"`
	MetricName string `json:"metricName"`
}

// ListPolicies returns autoscale policies.
func (a *AutoscaleAPI) ListPolicies(ctx context.Context) ([]ScalePolicy, error) {
	var out struct {
		Data []ScalePolicy `json:"data"`
	}
	return out.Data, a.c.get(ctx, "autoscale/policies", &out)
}

// PlacementAPI accesses placement policies.
type PlacementAPI struct{ c *Client }

// ListPolicies returns placement policies.
func (a *PlacementAPI) ListPolicies(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		Data []map[string]any `json:"data"`
	}
	return out.Data, a.c.get(ctx, "placement/policies", &out)
}

// ---- Instance types (capsule-types) ----------------------------------------

// InstanceTypesAPI accesses instance types (capsule types).
type InstanceTypesAPI struct{ c *Client }

// InstanceType is a capsule/instance type definition.
type InstanceType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Family      string `json:"family"`
	CPUCount    int    `json:"cpuCount"`
	MemoryBytes int64  `json:"memoryBytes"`
	GPUEligible bool   `json:"gpuEligible"`
}

// List returns instance types.
func (a *InstanceTypesAPI) List(ctx context.Context) ([]InstanceType, error) {
	var out struct {
		Data []InstanceType `json:"data"`
	}
	return out.Data, a.c.get(ctx, "capsule-types", &out)
}

// Get returns an instance type by name.
func (a *InstanceTypesAPI) Get(ctx context.Context, name string) (InstanceType, error) {
	var out struct {
		Data InstanceType `json:"data"`
	}
	return out.Data, a.c.get(ctx, "capsule-types/"+name, &out)
}

// ---- GPU -------------------------------------------------------------------

// GPUAPI accesses GPU inventory.
type GPUAPI struct{ c *Client }

// GPU is a registered GPU device.
type GPU struct {
	ID                 string `json:"id"`
	Vendor             string `json:"vendor"`
	Model              string `json:"model"`
	MemoryBytes        int64  `json:"memoryBytes"`
	Status             string `json:"status"`
	AssignedInstanceID string `json:"assignedInstanceId,omitempty"`
}

// List returns GPU inventory.
func (a *GPUAPI) List(ctx context.Context) ([]GPU, error) {
	var out struct {
		Data []GPU `json:"data"`
	}
	return out.Data, a.c.get(ctx, "gpu", &out)
}

// ---- Compute groups --------------------------------------------------------

// ComputeGroupsAPI accesses compute groups.
type ComputeGroupsAPI struct{ c *Client }

// ComputeGroup is a managed group of instances.
type ComputeGroup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TemplateID   string `json:"templateId"`
	TemplateName string `json:"templateName,omitempty"`
	MinSize      int    `json:"minSize"`
	DesiredSize  int    `json:"desiredSize"`
	MaxSize      int    `json:"maxSize"`
	Status       string `json:"status"`
}

// List returns compute groups.
func (a *ComputeGroupsAPI) List(ctx context.Context) ([]ComputeGroup, error) {
	var out struct {
		Data []ComputeGroup `json:"data"`
	}
	return out.Data, a.c.get(ctx, "groups", &out)
}

// Scale sets a compute group's desired size.
func (a *ComputeGroupsAPI) Scale(ctx context.Context, name string, desired int) error {
	return a.c.post(ctx, "groups/"+name+"/scale", map[string]int{"desired": desired}, nil)
}

// ---- Governance ------------------------------------------------------------

// GovernanceAPI accesses governance policies.
type GovernanceAPI struct{ c *Client }

// ListPolicies returns governance policies.
func (a *GovernanceAPI) ListPolicies(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		Data []map[string]any `json:"data"`
	}
	return out.Data, a.c.get(ctx, "governance/policies", &out)
}

// Evaluate runs a governance evaluation against a candidate object.
func (a *GovernanceAPI) Evaluate(ctx context.Context, body any) (map[string]any, error) {
	var out struct {
		Data map[string]any `json:"data"`
	}
	return out.Data, a.c.post(ctx, "governance/evaluate", body, &out)
}

// ---- Marketplace -----------------------------------------------------------

// MarketplaceAPI accesses the image marketplace.
type MarketplaceAPI struct{ c *Client }

// MarketplaceListing is a marketplace image listing.
type MarketplaceListing struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	ScanStatus string `json:"scanStatus"`
}

// List returns marketplace listings.
func (a *MarketplaceAPI) List(ctx context.Context) ([]MarketplaceListing, error) {
	var out struct {
		Data []MarketplaceListing `json:"data"`
	}
	return out.Data, a.c.get(ctx, "marketplace/images", &out)
}

// Get returns a listing by ID.
func (a *MarketplaceAPI) Get(ctx context.Context, id string) (MarketplaceListing, error) {
	var out struct {
		Data MarketplaceListing `json:"data"`
	}
	return out.Data, a.c.get(ctx, "marketplace/images/"+id, &out)
}

// ---- Node pools ------------------------------------------------------------

// NodePoolsAPI accesses node pools (topology).
type NodePoolsAPI struct{ c *Client }

// NodePool is a pool of nodes.
type NodePool struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MinNodes     int    `json:"minNodes"`
	DesiredNodes int    `json:"desiredNodes"`
	MaxNodes     int    `json:"maxNodes"`
	MemberCount  int    `json:"memberCount"`
}

// List returns node pools.
func (a *NodePoolsAPI) List(ctx context.Context) ([]NodePool, error) {
	var out struct {
		Data []NodePool `json:"data"`
	}
	return out.Data, a.c.get(ctx, "node-pools", &out)
}

// ---- Posture ---------------------------------------------------------------

// PostureAPI accesses security posture findings.
type PostureAPI struct{ c *Client }

// ListFindings returns posture findings.
func (a *PostureAPI) ListFindings(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		Data []map[string]any `json:"data"`
	}
	return out.Data, a.c.get(ctx, "posture/findings", &out)
}

// ---- CSD shared volumes ----------------------------------------------------

// CSDAPI accesses CSD shared/distributed volumes.
type CSDAPI struct{ c *Client }

// CSDVolume is a shared CSD volume.
type CSDVolume struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ListVolumes returns CSD volumes.
func (a *CSDAPI) ListVolumes(ctx context.Context) ([]CSDVolume, error) {
	var out struct {
		Data []CSDVolume `json:"data"`
	}
	return out.Data, a.c.get(ctx, "csd/volumes", &out)
}

// ---- VPC mobility (migrations) ---------------------------------------------

// MigrationsAPI accesses VPC mobility plans.
type MigrationsAPI struct{ c *Client }

// MigrationPlan is a VPC mobility plan.
type MigrationPlan struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// List returns migration plans.
func (a *MigrationsAPI) List(ctx context.Context) ([]MigrationPlan, error) {
	var out struct {
		Data []MigrationPlan `json:"data"`
	}
	return out.Data, a.c.get(ctx, "migrations", &out)
}

// ---- Health checks ---------------------------------------------------------

// HealthAPI accesses health checks.
type HealthAPI struct{ c *Client }

// ListChecks returns configured health checks.
func (a *HealthAPI) ListChecks(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		Data []map[string]any `json:"data"`
	}
	return out.Data, a.c.get(ctx, "health-checks", &out)
}

// ---- Backup policies -------------------------------------------------------

// BackupPoliciesAPI accesses scheduled backup policies.
type BackupPoliciesAPI struct{ c *Client }

// ListPolicies returns backup policies.
func (a *BackupPoliciesAPI) ListPolicies(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		Data []map[string]any `json:"data"`
	}
	return out.Data, a.c.get(ctx, "backup-policies", &out)
}
