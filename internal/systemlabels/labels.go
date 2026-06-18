package systemlabels

// Well-known instance labels for system-managed workloads.
const (
	Hidden        = "capper.system/hidden"
	Managed       = "capper.system/managed"
	ManagedDB     = "database"
	DatabaseID    = "capper.system/database-id"
)

// IsHidden reports whether an instance should be omitted from the Instances UI/API.
func IsHidden(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	if labels[Hidden] == "true" {
		return true
	}
	return labels[Managed] == ManagedDB
}
