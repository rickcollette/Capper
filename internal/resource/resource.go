// Package resource defines the common metadata envelope shared by every Capper
// managed resource, plus ID generation, label matching, and a SQLite-backed
// store. All service-layer managers embed or reference Resource rows.
package resource

// Resource is the common metadata envelope for every Capper managed object.
type Resource struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Project     string            `json:"project"`
	Owner       string            `json:"owner,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Status      string            `json:"status"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// Resource type constants used as the Type field and as ID prefixes.
const (
	TypeInstance     = "instance"
	TypeImage        = "image"
	TypeNetwork      = "network"
	TypeVolume       = "volume"
	TypeLoadBalancer = "load-balancer"
	TypeSecret       = "secret"
	TypeDatabase     = "database"
	TypeAgent        = "agent"
)

// Status values shared across resource types.
const (
	StatusCreating = "creating"
	StatusActive   = "active"
	StatusUpdating = "updating"
	StatusDeleting = "deleting"
	StatusDeleted  = "deleted"
	StatusFailed   = "failed"
)
