package adminconfig

// Host-wide limit keys. These are admin-configurable caps that apply to the
// whole host/control plane (as opposed to per-account quotas). An unset key
// means "use the built-in default" — e.g. host.deployments.max falls back to
// the RAM-derived deploy cap.
const (
	// KeyHostDeploymentsMax overrides the host-wide capsule deployment cap
	// (user instances + system-managed workloads). Unset => derived default.
	KeyHostDeploymentsMax = "host.deployments.max"

	// KeyDefaultInstancePool is the storage pool ID instance disks are drawn
	// from. Unset => instance disks live under the store path (legacy behavior).
	KeyDefaultInstancePool = "storage.instance_pool"
)

// HostLimitKeys is the ordered set of host-limit keys the Admin UI manages.
var HostLimitKeys = []string{
	KeyHostDeploymentsMax,
}
