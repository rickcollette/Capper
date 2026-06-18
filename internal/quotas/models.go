package quotas

const (
	KeyComputeInstancesMax   = "compute.instances.max"
	KeyComputeVCPUMax        = "compute.vcpu.max"
	KeyComputeMemoryBytesMax = "compute.memory_bytes.max"
	KeyStorageBucketsMax     = "storage.buckets.max"
	KeyStorageBytesMax       = "storage.bytes.max"
	KeyVPCCountMax           = "vpc.count.max"
	KeyLBCountMax            = "lb.count.max"
	KeyDNSZonesMax           = "dns.zones.max"
	KeyIAMUsersMax           = "iam.users.max"
	KeyIAMRolesMax           = "iam.roles.max"
	KeyCertsMax              = "certificates.max"
)

// DefaultQuotas maps resource type keys to their default limits.
var DefaultQuotas = map[string]int{
	KeyComputeInstancesMax:   100,
	KeyComputeVCPUMax:        1000,
	KeyComputeMemoryBytesMax: 10 * 1024 * 1024 * 1024 * 1024, // 10 TiB
	KeyStorageBucketsMax:     50,
	KeyStorageBytesMax:       100 * 1024 * 1024 * 1024 * 1024, // 100 TiB
	KeyVPCCountMax:           20,
	KeyLBCountMax:            50,
	KeyDNSZonesMax:           100,
	KeyIAMUsersMax:           500,
	KeyIAMRolesMax:           200,
	KeyCertsMax:              100,
}

// Quota represents the configured limit for a resource type within an account.
type Quota struct {
	AccountID    string `json:"accountId"`
	ResourceType string `json:"resourceType"`
	Limit        int64  `json:"limit"`
}

// Usage represents the tracked consumption of a resource within an account.
type Usage struct {
	AccountID    string `json:"accountId"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	MetricName   string `json:"metricName"`
	Value        int64  `json:"value"`
}
