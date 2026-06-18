package store

import "capper/internal/deploylimit"

// CheckHostDeployLimit returns an error when the host has reached its combined
// capsule deployment cap (user instances + system-managed workloads).
func (s *Store) CheckHostDeployLimit() error {
	instances, err := s.ListInstances()
	if err != nil {
		return nil
	}
	return deploylimit.CheckCount(int64(len(instances)))
}
