package store

// liveInstanceIDs returns the set of instance IDs that currently exist.
func (s *Store) liveInstanceIDs() (map[string]bool, error) {
	insts, err := s.ListInstances()
	if err != nil {
		return nil, err
	}
	live := make(map[string]bool, len(insts))
	for _, i := range insts {
		live[i.ID] = true
	}
	return live, nil
}

// PruneOrphanedNetworkLeases removes network leases whose owning instance no
// longer exists. Stale leases otherwise accumulate when an instance is removed
// without its lease being released, blocking network deletion and IP reuse.
// Returns the number of leases removed.
func (s *Store) PruneOrphanedNetworkLeases() (int, error) {
	live, err := s.liveInstanceIDs()
	if err != nil {
		return 0, err
	}
	leases, err := s.Networks.AllLeases()
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, l := range leases {
		if live[l.InstanceID] {
			continue
		}
		if err := s.Networks.DeleteLease(l.NetworkID, l.InstanceID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// NetworkAttachment describes an instance still holding a lease on a network.
type NetworkAttachment struct {
	InstanceID   string
	InstanceName string
	IP           string
	Status       string
	Running      bool
}

// LiveNetworkAttachments returns the instances that still exist and hold a lease
// on the given network. It first prunes any orphaned leases (instances that no
// longer exist), so the result reflects only real, blocking attachments.
func (s *Store) LiveNetworkAttachments(networkID string) ([]NetworkAttachment, error) {
	if _, err := s.PruneOrphanedNetworkLeases(); err != nil {
		return nil, err
	}
	leases, err := s.Networks.LeasesForNetwork(networkID)
	if err != nil {
		return nil, err
	}
	byID := map[string]*NetworkAttachment{}
	var out []NetworkAttachment
	for _, l := range leases {
		inst, err := s.ResolveInstance(l.InstanceID)
		if err != nil || inst == nil {
			continue // pruned above, but be defensive
		}
		a := NetworkAttachment{
			InstanceID:   inst.ID,
			InstanceName: inst.Name,
			IP:           l.IP,
			Status:       string(inst.Status),
			Running:      inst.Status == "running",
		}
		if _, seen := byID[inst.ID]; !seen {
			out = append(out, a)
			byID[inst.ID] = &a
		}
	}
	return out, nil
}
