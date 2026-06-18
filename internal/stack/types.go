package stack

type StackStatus string

const (
	StackActive    StackStatus = "active"
	StackDestroyed StackStatus = "destroyed"
)

type StackResource struct {
	Type string `json:"type"` // "instance", "network", "lb", "dns_zone", "dns_record", "firewall"
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Stack struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Project      string        `json:"project"`
	TemplateHash string        `json:"templateHash"`
	Status       StackStatus   `json:"status"`
	Resources    []StackResource `json:"resources"`
	CreatedAt    string        `json:"createdAt"`
	UpdatedAt    string        `json:"updatedAt"`
}

type PlanOp struct {
	Action string `json:"action"` // "create", "update", "delete"
	Type   string `json:"type"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
