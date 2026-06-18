package store

import (
	"capper/internal/resourcemon"
)

// healthFromStatus maps a service status string to a resource-monitor health value.
func healthFromStatus(status string) string { return resourcemon.DeriveHealth(status) }

// SyncResourceMonitor projects the authoritative resource stores (instances,
// networks, nodes, load balancers) into the unified resource-monitor inventory.
// It reconciles each type: present resources are upserted; vanished ones are
// soft-deleted. Returns one SyncResult per type.
func (s *Store) SyncResourceMonitor(project string) ([]resourcemon.SyncResult, error) {
	mgr := resourcemon.NewManager(s.ResourceMon)
	var results []resourcemon.SyncResult

	// Instances.
	instances, err := s.ListInstances()
	if err != nil {
		return results, err
	}
	instItems := make([]resourcemon.Resource, 0, len(instances))
	for _, in := range instances {
		instItems = append(instItems, resourcemon.Resource{
			Name:       in.Name,
			Project:    project,
			RealmID:    in.RealmID,
			RegionID:   in.RegionID,
			ZoneID:     in.ZoneID,
			NodeID:     in.NodeID,
			Status:     in.Status,
			Health:     healthFromStatus(in.Status),
			Labels:     in.Labels,
			LastSeenAt: in.StartedAt,
		})
	}
	r, err := mgr.SyncType("instance", instItems)
	if err != nil {
		return results, err
	}
	results = append(results, r)

	// Networks.
	networks, err := s.Networks.List(project)
	if err != nil {
		return results, err
	}
	netItems := make([]resourcemon.Resource, 0, len(networks))
	for _, n := range networks {
		netItems = append(netItems, resourcemon.Resource{
			Name:    n.Name,
			Project: n.Project,
			Status:  n.Status,
			Health:  healthFromStatus(n.Status),
			Labels:  n.Labels,
		})
	}
	r, err = mgr.SyncType("network", netItems)
	if err != nil {
		return results, err
	}
	results = append(results, r)

	// Nodes (topology) — global, not project-scoped.
	nodes, err := s.Topology.Store().ListNodes("")
	if err != nil {
		return results, err
	}
	nodeItems := make([]resourcemon.Resource, 0, len(nodes))
	for _, n := range nodes {
		status := n.Status
		if n.Cordoned {
			status = "cordoned"
		}
		nodeItems = append(nodeItems, resourcemon.Resource{
			Name:       n.Slug,
			RealmID:    n.RealmID,
			RegionID:   n.RegionID,
			ZoneID:     n.ZoneID,
			NodeID:     n.ID,
			Status:     status,
			Health:     healthFromStatus(n.Status),
			Labels:     n.Labels,
			LastSeenAt: n.LastHeartbeat,
		})
	}
	r, err = mgr.SyncType("node", nodeItems)
	if err != nil {
		return results, err
	}
	results = append(results, r)

	// Load balancers.
	lbs, err := s.LB.List(project)
	if err == nil { // LB listing is best-effort; absence shouldn't fail the sync.
		lbItems := make([]resourcemon.Resource, 0, len(lbs))
		for _, lb := range lbs {
			lbItems = append(lbItems, resourcemon.Resource{
				Name:    lb.Name,
				Project: project,
				Status:  string(lb.Status),
				Health:  healthFromStatus(string(lb.Status)),
			})
		}
		r, err = mgr.SyncType("load-balancer", lbItems)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}

	return results, nil
}
