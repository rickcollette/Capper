package types

import "fmt"

// CascadeDeleteError describes a failure during cascade deletion of a resource.
// It includes the resource type, ID, the step that failed, the underlying cause,
// and whether the error is recoverable (soft failure) or blocking (hard failure).
type CascadeDeleteError struct {
	Resource    string
	ID          string
	Step        string // "stop-instance", "disconnect-network", etc.
	Cause       error
	Recoverable bool   // if false, deletion is aborted; if true, can proceed
	Recovery    string // actionable recovery suggestion
}

func (e *CascadeDeleteError) Error() string {
	return fmt.Sprintf("cascade delete %s:%s at step %q: %v (recoverable=%v)",
		e.Resource, e.ID, e.Step, e.Cause, e.Recoverable)
}

// CascadeDeletePlan describes the sequence of steps to delete a resource and
// its dependent resources. Used for pre-flight checks before actual deletion.
type CascadeDeletePlan struct {
	RootResource string         // "vpc", "load-balancer", "firewall", etc.
	RootID       string         // resource ID
	Steps        []CascadeStep  // deletion steps in order
	Blockers     []CascadeBlocker // resources that must be deleted manually
}

// CascadeStep represents one step in a cascade deletion sequence.
type CascadeStep struct {
	Sequence  int      // 1, 2, 3, ...
	Resource  string   // "instance", "subnet", "security-group", etc.
	ID        string   // resource ID
	Action    string   // "stop", "disconnect", "delete"
	DependsOn []string // IDs of resources that must be deleted first
}

// CascadeBlocker represents a resource that blocks deletion and must be
// handled manually before the cascade can proceed.
type CascadeBlocker struct {
	Resource string // "instance", "load-balancer", "network-attachment"
	ID       string
	Reason   string // "running", "has-targets", "in-use"
	Recovery string // "stop the instance first", "remove targets before deletion"
}
