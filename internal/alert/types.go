package alert

// RuleType determines what is evaluated.
type RuleType string

const (
	RuleTypeEventCount  RuleType = "event_count"  // fires when event count >= threshold in window
	RuleTypeMetricThreshold RuleType = "metric_threshold" // fires when metric value >= threshold
)

// Rule is a stored alert rule.
type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Project     string   `json:"project"`
	Type        RuleType `json:"type"`
	// For event_count: action prefix to match, window seconds, threshold count.
	EventAction string `json:"eventAction,omitempty"`
	WindowSecs  int    `json:"windowSecs,omitempty"`
	Threshold   int    `json:"threshold"`
	// For metric_threshold: metric name (cpu_micros, memory_bytes, pid_count).
	MetricName  string `json:"metricName,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
}

// Firing represents an alert that has crossed its threshold.
type Firing struct {
	RuleID    string `json:"ruleId"`
	RuleName  string `json:"ruleName"`
	Value     int64  `json:"value"`
	Threshold int    `json:"threshold"`
	FiredAt   string `json:"firedAt"`
}
