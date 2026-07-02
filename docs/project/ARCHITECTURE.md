# Capper Architecture Guide

## Data Storage Consolidation Status

### VPC Storage Pattern (Intentional Split)

VPC data is stored in **two places by design**, each with distinct purposes:

#### 1. Topology Store (`internal/topology/store.go`)
- **Canonical source** for VPC identity and metadata
- Accessed via: `s.ctrl.Store.Topology.Store().GetVPC(project, slug)`
- Stores: name, slug, CIDR block, labels, status, mobility policy
- Used by: `handlers_topology.go` (CRUD operations), `vpcmover` (VPC relocation)
- Accessed in: API endpoints for VPC management, CLI commands

#### 2. VPC Store (`internal/vpc/store.go`)
- **Supporting infrastructure** for VPC networking details
- Accessed via: `s.ctrl.Store.VPC.GetVPC(vpcID, project)`
- Stores: subnets, route tables, network ACLs, security groups, ENIs, peerings
- Used by: `handlers_vpc.go` (network configuration), internal VPC operations
- Accessed in: API endpoints for networking, instance launch

### Why Both Stores?

- **Topology** tracks the VPC as a logical resource (what users interact with)
- **VPC** tracks the VPC as a network container (what instances and services use)
- Separation allows independent scaling: VPC operations are frequent, topology changes are rare

### Deletion Implications

When deleting a VPC, both stores must be updated:
```go
// Step 1: Delete from vpc store (cascades networking resources)
s.ctrl.Store.VPC.DeleteVPC(vpcID, project)

// Step 2: Delete from topology store (removes logical resource)
s.ctrl.Store.Topology.Store().DeleteVPC(project, vpc.Slug)
```

This is handled in `asyncDeleteVPC()` in `handlers_deletion.go` (lines 291-299).

### CRUD Operation Locations

| Operation | Store | Location |
|-----------|-------|----------|
| Create VPC | Topology | `handlers_topology.go:484` |
| Get VPC | Topology | `handlers_topology.go:493` |
| List VPCs | Topology | `handlers_topology.go:469` |
| Update VPC | Topology | `handlers_topology.go:532` |
| Delete VPC | Both | `handlers_deletion.go:291` (Topology first), `:293` (VPC second) |
| Network Config | VPC | `handlers_vpc.go` |

---

## Resource Storage Consolidation (Future Work)

### Current State
- **Instances**: Single canonical store in `internal/store/instances.go` ✅
- **Load Balancers**: Single canonical store in `internal/lb/store.go` ✅
- **Databases**: Single canonical store in `internal/store/databases.go` ✅ (inferred)
- **VPCs**: Dual stores (Topology + VPC package) - intentional by design ⚠️

### Dead Code Removed
- `lb_target_groups` and `lb_listeners` table definitions removed from `vpc/store_migrate.go`
  - These were accidentally defined in VPC package but belong in LB package
  - Never accessed from VPC code
  - Removed in consolidation phase 1

### Recommendations for Future Consolidation
1. Periodically audit for similar orphaned table definitions
2. Consider eventually merging Topology and VPC stores if use cases align
3. Maintain clear ownership: one store per logical resource

