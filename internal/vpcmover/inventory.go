package vpcmover

import (
	"capper/internal/topology"
)

// InventoryStore holds the stores needed to build a live VPC inventory.
type InventoryStore struct {
	Topology *topology.Store
}

// BuildInventory queries the topology store to populate a VPCInventory.
// If the VPC cannot be found the function falls back to a minimal stub so that
// planning can still proceed (the planner will emit a CodeSourceNotFound error).
func BuildInventory(store InventoryStore, vpcID string) (*VPCInventory, error) {
	inv := &VPCInventory{VPCID: vpcID}

	if store.Topology == nil {
		return inv, nil
	}

	// Try to look up the VPC by ID (project="" means search all projects).
	vpc, err := store.Topology.GetVPC("", vpcID)
	if err != nil {
		// VPC not found — return minimal inventory; caller decides severity.
		return inv, nil
	}

	// Populate basic VPC fields we can derive from the record.
	_ = vpc // VPCID is already set; future fields (CIDR, etc.) could be added here.

	// Count subnets.
	subnets, err := store.Topology.ListSubnets(vpcID)
	if err == nil {
		inv.SubnetCount = len(subnets)
	}

	// Count routes.
	routes, err := store.Topology.ListRoutes(vpcID)
	if err == nil {
		inv.RouteCount = len(routes)
	}

	return inv, nil
}
