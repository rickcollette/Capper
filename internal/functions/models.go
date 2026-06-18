// Package functions implements Capper Functions — a Lambda-style serverless
// subsystem. Functions are versioned units of code/image invoked synchronously
// (HTTP/manual) or asynchronously via event triggers (queue, schedule). Each
// invocation is recorded with status, duration, and result reference.
package functions

// Function is a deployable serverless unit.
type Function struct {
	ID          string            `json:"id"`
	Project     string            `json:"project"`
	Name        string            `json:"name"`
	Runtime     string            `json:"runtime"`
	Handler     string            `json:"handler,omitempty"`
	Image       string            `json:"image,omitempty"`
	Command     []string          `json:"command,omitempty"`
	PackageID   string            `json:"packageId,omitempty"`
	Version     string            `json:"version"`
	Status      string            `json:"status"`
	MemoryBytes int64             `json:"memoryBytes"`
	CPUUnits    int               `json:"cpuUnits"`
	TimeoutMS   int               `json:"timeoutMs"`
	Concurrency int               `json:"concurrency"`
	MinScale    int               `json:"minScale"`
	MaxScale    int               `json:"maxScale"`
	Isolation   string            `json:"isolation"`
	Env         map[string]string `json:"env,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// FunctionVersion is an immutable published version of a function.
type FunctionVersion struct {
	ID         string `json:"id"`
	FunctionID string `json:"functionId"`
	Version    string `json:"version"`
	PackageID  string `json:"packageId,omitempty"`
	Image      string `json:"image,omitempty"`
	ConfigJSON string `json:"config,omitempty"`
	Digest     string `json:"digest,omitempty"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

// Trigger binds an event source to a function.
type Trigger struct {
	ID              string            `json:"id"`
	Project         string            `json:"project"`
	FunctionID      string            `json:"functionId"`
	Type            string            `json:"type"`   // http, queue, schedule, bucket-event
	Source          string            `json:"source"` // queue name, cron expr, bucket name
	PatternJSON     string            `json:"pattern,omitempty"`
	RetryPolicyJSON string            `json:"retryPolicy,omitempty"`
	Enabled         bool              `json:"enabled"`
	Labels          map[string]string `json:"labels,omitempty"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
}

// Invocation records one execution of a function.
type Invocation struct {
	ID              string `json:"id"`
	Project         string `json:"project"`
	FunctionID      string `json:"functionId"`
	FunctionVersion string `json:"functionVersion,omitempty"`
	TriggerID       string `json:"triggerId,omitempty"`
	RequestID       string `json:"requestId,omitempty"`
	Principal       string `json:"principal,omitempty"`
	Source          string `json:"source,omitempty"`
	Status          string `json:"status"`
	StartedAt       string `json:"startedAt,omitempty"`
	EndedAt         string `json:"endedAt,omitempty"`
	DurationMS      int64  `json:"durationMs"`
	Error           string `json:"error,omitempty"`
	Result          string `json:"result,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// Status constants.
const (
	StatusCreated = "created"
	StatusReady   = "ready"
	StatusError   = "error"

	InvocationPending   = "pending"
	InvocationRunning   = "running"
	InvocationSucceeded = "succeeded"
	InvocationFailed    = "failed"
	InvocationTimeout   = "timeout"
)

// Default resource limits.
const (
	DefaultMemoryBytes = 268435456 // 256 MiB
	DefaultCPUUnits    = 250
	DefaultTimeoutMS   = 30000
	DefaultConcurrency = 10
	DefaultMaxScale    = 10
)
