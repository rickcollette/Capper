package autoscale

import "errors"

// PolicyType controls the scaling algorithm.
const (
	PolicyTypeTarget    = "target"
	PolicyTypeThreshold = "threshold"
	PolicyTypeStep      = "step"
	PolicyTypeSchedule  = "schedule"
	PolicyTypeQueue     = "queue"
)

// MetricScope defines what a metric is scoped to.
const (
	ScopeInstance     = "instance"
	ScopeGroup        = "group"
	ScopeLoadBalancer = "load-balancer"
	ScopeQueue        = "queue"
	ScopeNode         = "node"
	ScopeCustom       = "custom"
)

// Decision values written to autoscale_decisions.
const (
	DecisionScaleOut = "scale-out"
	DecisionScaleIn  = "scale-in"
	DecisionHold     = "hold"
	DecisionBlocked  = "blocked"
	DecisionDisabled = "disabled"
	DecisionError    = "error"
)

// Well-known metric names.
const (
	MetricCPUAvgPercent        = "group_cpu_avg_percent"
	MetricMemoryAvgPercent     = "group_memory_avg_percent"
	MetricActiveConnsPerInst   = "group_active_connections_per_instance"
	MetricRequestsPerSec       = "group_requests_per_second"
	MetricHealthyReplicas      = "group_healthy_replicas"
	MetricQueueDepth           = "queue_depth"
	MetricOldestJobAgeSecs     = "oldest_job_age_seconds"
)

// AutoscalePolicy defines when and how to scale an instance group.
type AutoscalePolicy struct {
	ID          string `json:"id"`
	Project     string `json:"project"`
	Name        string `json:"name"`
	GroupID     string `json:"groupId"`
	GroupName   string `json:"groupName,omitempty"`
	Enabled     bool   `json:"enabled"`

	PolicyType  string  `json:"policyType"`
	MetricName  string  `json:"metricName"`
	MetricScope string  `json:"metricScope"`
	QueueName   string  `json:"queueName,omitempty"`

	TargetValue       float64 `json:"targetValue"`
	ScaleOutThreshold float64 `json:"scaleOutThreshold"`
	ScaleInThreshold  float64 `json:"scaleInThreshold"`

	MinReplicas int `json:"minReplicas"`
	MaxReplicas int `json:"maxReplicas"`
	ScaleOutStep int `json:"scaleOutStep"`
	ScaleInStep  int `json:"scaleInStep"`

	ScaleOutCooldownSecs int `json:"scaleOutCooldownSeconds"`
	ScaleInCooldownSecs  int `json:"scaleInCooldownSeconds"`
	EvalWindowSecs       int `json:"evaluationWindowSeconds"`
	StabWindowSecs       int `json:"stabilizationWindowSeconds"`

	// ScheduleJSON holds []ScheduleEntry when PolicyType == "schedule".
	ScheduleJSON string `json:"scheduleJson,omitempty"`

	LastScaleAt  string `json:"lastScaleAt"`
	LastDecision string `json:"lastDecision"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// ScheduleEntry is one rule in a schedule-based policy.
type ScheduleEntry struct {
	Cron            string `json:"cron"`
	DesiredReplicas int    `json:"desiredReplicas"`
}

// AutoscaleDecision is an immutable audit record of one scaling evaluation.
type AutoscaleDecision struct {
	ID                  string  `json:"id"`
	PolicyID            string  `json:"policyId"`
	GroupID             string  `json:"groupId"`
	Project             string  `json:"project"`
	OldReplicas         int     `json:"oldReplicas"`
	NewReplicas         int     `json:"newReplicas"`
	RecommendedReplicas int     `json:"recommendedReplicas"`
	Decision            string  `json:"decision"`
	Reason              string  `json:"reason"`
	MetricName          string  `json:"metricName"`
	MetricValue         float64 `json:"metricValue"`
	TargetValue         float64 `json:"targetValue"`
	Blocked             bool    `json:"blocked"`
	BlockedReason       string  `json:"blockedReason"`
	CreatedAt           string  `json:"createdAt"`
}

// MetricSample is one observation from any metric source.
type MetricSample struct {
	ID         string            `json:"id"`
	Project    string            `json:"project"`
	Scope      string            `json:"scope"`
	ResourceID string            `json:"resourceId"`
	MetricName string            `json:"metricName"`
	Value      float64           `json:"value"`
	Labels     map[string]string `json:"labels,omitempty"`
	SampledAt  string            `json:"sampledAt"`
}

// ScaleRecommendation is the output of an Evaluator.Evaluate call.
type ScaleRecommendation struct {
	Decision            string
	OldReplicas         int
	RecommendedReplicas int
	NewReplicas         int
	MetricName          string
	MetricValue         float64
	TargetValue         float64
	Reason              string
	Blocked             bool
	BlockedReason       string
}

// GroupMetrics are derived, per-group metrics computed from individual instance metrics.
type GroupMetrics struct {
	GroupID            string
	CPUAvgPercent      float64
	MemoryAvgPercent   float64
	ActiveConnsPerInst float64
	RequestsPerSec     float64
	HealthyReplicas    int
	TotalReplicas      int
}

// ErrNotFound is returned when a policy or decision record is not found.
var ErrNotFound = errors.New("autoscale: not found")
