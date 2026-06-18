package store

import (
	"capper/internal/adminconfig"
	"capper/internal/deploylimit"
)

// HostDeploymentCap returns the effective host-wide capsule deployment cap: a
// positive admin-configured override (admin_config) wins; otherwise the
// env/RAM-derived default applies.
func (s *Store) HostDeploymentCap() int64 {
	var override int64
	if s.AdminConfig != nil {
		if n, ok, err := s.AdminConfig.GetInt(adminconfig.KeyHostDeploymentsMax); err == nil && ok {
			override = n
		}
	}
	return deploylimit.ResolveMax(override)
}

// CheckHostDeployLimit returns an error when the host has reached its combined
// capsule deployment cap (user instances + system-managed workloads).
func (s *Store) CheckHostDeployLimit() error {
	instances, err := s.ListInstances()
	if err != nil {
		return nil
	}
	return deploylimit.CheckCountWithMax(int64(len(instances)), s.HostDeploymentCap())
}
