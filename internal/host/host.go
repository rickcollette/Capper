// Package host manages the inventory of hosts running Capper and provides
// capability checks (doctor) for the local node.
package host

// Host represents a node in the Capper cluster.
type Host struct {
	ID            string            `json:"id"`
	Hostname      string            `json:"hostname"`
	Roles         []string          `json:"roles,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	OS            string            `json:"os"`
	Arch          string            `json:"arch"`
	KernelVersion string            `json:"kernelVersion"`
	CPUCount      int               `json:"cpuCount"`
	MemoryBytes   int64             `json:"memoryBytes"`
	Addresses     []string          `json:"addresses,omitempty"`
	Status        string            `json:"status"`
	RegisteredAt  string            `json:"registeredAt"`
	LastSeen      string            `json:"lastSeen"`
}

// Host status values.
const (
	StatusReady   = "ready"
	StatusDrained = "drained"
	StatusOffline = "offline"
)

// DoctorResult is the outcome of a single capability check.
type DoctorResult struct {
	Check   string `json:"check"`
	Pass    bool   `json:"pass"`
	Message string `json:"message"`
}
