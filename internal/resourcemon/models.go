// Package resourcemon implements the Capper Resource Monitor (capper-observe):
// a unified resource registry, configuration history with drift detection,
// metrics, resource events, and alerting across every major Capper service.
//
// It owns its own tables (prefixed rmon_) and projects existing resources
// (instances, networks, VPCs, firewalls, load balancers, DNS zones,
// certificates, nodes) into a single searchable inventory via sync adapters.
// Audit events are recorded through the existing internal/audit package.
package resourcemon

// Resource is one row in the unified inventory.
type Resource struct {
	ID                string            `json:"id"`
	ResourceType      string            `json:"resourceType"`
	Name              string            `json:"name"`
	Project           string            `json:"project,omitempty"`
	AccountID         string            `json:"accountId,omitempty"`
	RealmID           string            `json:"realmId,omitempty"`
	RegionID          string            `json:"regionId,omitempty"`
	ZoneID            string            `json:"zoneId,omitempty"`
	NodeID            string            `json:"nodeId,omitempty"`
	Status            string            `json:"status"`
	Health            string            `json:"health"`
	Owner             string            `json:"owner,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`
	ConfigurationHash string            `json:"configurationHash,omitempty"`
	LastSeenAt        string            `json:"lastSeenAt,omitempty"`
	CreatedAt         string            `json:"createdAt"`
	UpdatedAt         string            `json:"updatedAt"`
	DeletedAt         string            `json:"deletedAt,omitempty"`
}

// ResourceConfig is one version of a resource's desired/observed configuration.
type ResourceConfig struct {
	ID                 string `json:"id"`
	ResourceID         string `json:"resourceId"`
	Version            int    `json:"version"`
	DesiredConfigJSON  string `json:"desiredConfig"`
	ObservedConfigJSON string `json:"observedConfig"`
	LastAppliedJSON    string `json:"lastAppliedConfig"`
	ConfigHash         string `json:"configHash"`
	DriftStatus        string `json:"driftStatus"`
	DriftReason        string `json:"driftReason,omitempty"`
	CreatedBy          string `json:"createdBy,omitempty"`
	CreatedAt          string `json:"createdAt"`
}

// MetricSample is a single time-series data point.
type MetricSample struct {
	ID           string            `json:"id"`
	Project      string            `json:"project,omitempty"`
	ResourceType string            `json:"resourceType"`
	ResourceID   string            `json:"resourceId"`
	MetricName   string            `json:"metricName"`
	Value        float64           `json:"value"`
	Unit         string            `json:"unit,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	SampledAt    string            `json:"sampledAt"`
}

// ResourceEvent is a lifecycle/operational event for a resource.
type ResourceEvent struct {
	ID           string `json:"id"`
	Project      string `json:"project,omitempty"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventType    string `json:"eventType"`
	Severity     string `json:"severity"`
	Message      string `json:"message,omitempty"`
	DetailsJSON  string `json:"details,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

// AlertRule defines a metric condition that opens alerts when breached.
type AlertRule struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Project            string  `json:"project,omitempty"`
	ResourceType       string  `json:"resourceType,omitempty"`
	MetricName         string  `json:"metricName,omitempty"`
	Condition          string  `json:"condition"` // gt, gte, lt, lte, eq, ne
	Threshold          float64 `json:"threshold"`
	DurationSeconds    int     `json:"durationSeconds"`
	Severity           string  `json:"severity"`
	Enabled            bool    `json:"enabled"`
	NotificationTarget string  `json:"notificationTarget,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

// Alert is an open/acknowledged/resolved alert instance.
type Alert struct {
	ID             string `json:"id"`
	RuleID         string `json:"ruleId,omitempty"`
	Project        string `json:"project,omitempty"`
	ResourceType   string `json:"resourceType"`
	ResourceID     string `json:"resourceId"`
	Severity       string `json:"severity"`
	Status         string `json:"status"` // open, acknowledged, resolved
	Title          string `json:"title"`
	Message        string `json:"message"`
	OpenedAt       string `json:"openedAt"`
	AcknowledgedAt string `json:"acknowledgedAt,omitempty"`
	ResolvedAt     string `json:"resolvedAt,omitempty"`
}

// Drift status values.
const (
	DriftUnknown    = "unknown"
	DriftInSync     = "in_sync"
	DriftDrifted    = "drifted"
	DriftErrored    = "errored"
)

// Health values.
const (
	HealthUnknown   = "unknown"
	HealthHealthy   = "healthy"
	HealthDegraded  = "degraded"
	HealthUnhealthy = "unhealthy"
)

// DeriveHealth maps a service status string to a resource health value.
func DeriveHealth(status string) string {
	switch status {
	case "running", "ready", "active", "healthy", "available":
		return HealthHealthy
	case "degraded", "warning":
		return HealthDegraded
	case "error", "failed", "stopped", "unreachable", "down":
		return HealthUnhealthy
	default:
		return HealthUnknown
	}
}

// ResourceFilter holds optional query filters for ListResources.
type ResourceFilter struct {
	Project      string
	ResourceType string
	Status       string
	Health       string
	RegionID     string
	ZoneID       string
	NodeID       string
	Query        string // substring match on name
	Limit        int
}

// MetricQuery selects metric samples for a resource/metric over a time range.
type MetricQuery struct {
	ResourceType string
	ResourceID   string
	MetricName   string
	Since        string // RFC3339; empty = no lower bound
	Limit        int
}
