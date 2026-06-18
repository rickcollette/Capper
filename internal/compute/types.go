package compute

// Host is a machine capable of running Capper workloads.
type Host struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Address     string            `json:"address"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	CPUCount    int               `json:"cpuCount"`
	MemoryBytes int64             `json:"memoryBytes"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

const (
	HostStatusReady    = "ready"
	HostStatusDrained  = "drained"
	HostStatusCordoned = "cordoned"
)

// Template defines how to launch repeatable workloads.
type Template struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Image     string      `json:"image"`
	Runtime   string      `json:"runtime"`
	Doc       TemplateDoc `json:"document"`
	CreatedAt string      `json:"createdAt"`
}

// TemplateDoc is the rich JSON document stored inside a Template.
type TemplateDoc struct {
	Name             string            `json:"name"`
	Image            string            `json:"image"`
	Runtime          string            `json:"runtime,omitempty"`
	Resources        ResourceSpec      `json:"resources,omitempty"`
	InstanceTypeName string            `json:"instanceType,omitempty"`
	Network          TemplateNetwork   `json:"network,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Health           HealthCheck       `json:"health,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
}

// ResourceSpec carries resource limits used by a template.
type ResourceSpec struct {
	MemoryBytes  int64 `json:"memoryBytes,omitempty"`
	MaxProcesses int64 `json:"maxProcesses,omitempty"`
	CPUTimeSecs  int64 `json:"cpuTimeSecs,omitempty"`
}

// TemplateNetwork names the virtual network and aliases for instances launched
// from a template.
type TemplateNetwork struct {
	Name    string   `json:"name,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

// HealthCheck describes how to probe an instance's liveness.
type HealthCheck struct {
	Type         string `json:"type,omitempty"`
	Path         string `json:"path,omitempty"`
	Port         int    `json:"port,omitempty"`
	IntervalSecs int    `json:"intervalSecs,omitempty"`
}

// Group keeps N copies of a workload running.
type Group struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TemplateID   string `json:"templateId"`
	TemplateName string `json:"templateName,omitempty"`
	MinSize      int    `json:"minSize"`
	DesiredSize  int    `json:"desiredSize"`
	MaxSize      int    `json:"maxSize"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
}

const (
	GroupStatusActive   = "active"
	GroupStatusScaling  = "scaling"
	GroupStatusDraining = "draining"
)

// Snapshot is a point-in-time capture of an instance's rootfs.
type Snapshot struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	InstanceID string `json:"instanceId"`
	Path       string `json:"path"`
	Digest     string `json:"digest"`
	CreatedAt  string `json:"createdAt"`
}

// InstanceType defines a locked resource envelope for capsule instances.
type InstanceType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`   // cap-m2, cap-c3, cap-g1
	Family      string `json:"family"` // memory, compute, gpu
	CPUCount    int    `json:"cpuCount"`
	MemoryBytes int64  `json:"memoryBytes"`
	DiskBytes   int64  `json:"diskBytes"`
	PIDLimit    int    `json:"pidLimit"`
	GPUEligible bool   `json:"gpuEligible"`
	GPUCount    int    `json:"gpuCount"`
	Locked      bool   `json:"locked"` // built-in types cannot be deleted
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

const (
	InstanceTypeFamilyMemory   = "memory"
	InstanceTypeFamilyCompute  = "compute"
	InstanceTypeFamilyGPU      = "gpu"
	InstanceTypeFamilyStandard = "standard"
)

// GPUDevice is a GPU available for passthrough assignment.
type GPUDevice struct {
	ID                 string `json:"id"`
	Vendor             string `json:"vendor"`
	Model              string `json:"model"`
	MemoryBytes        int64  `json:"memoryBytes"`
	Status             string `json:"status"`
	DevicePath         string `json:"devicePath,omitempty"`
	AssignedInstanceID string `json:"assignedInstanceId,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

const (
	GPUStatusAvailable = "available"
	GPUStatusAssigned  = "assigned"
	GPUStatusDrained   = "drained"
)

// RunSpec is returned by RunFromTemplate so callers know what image to launch.
type RunSpec struct {
	TemplateName     string
	Image            string
	Resources        ResourceSpec
	InstanceName     string
	InstanceTypeName string // set when the template specifies an instance type
}

// ReconcileResult summarises the outcome of a group reconcile pass.
type ReconcileResult struct {
	GroupID string
	Desired int
	Actual  int
	Created []string
	Removed []string
	Errors  []string
}

// RunFunc is injected into Manager.Reconcile and RunFromTemplate so that
// the compute package never imports manager or controller (avoids import cycles
// with store).
type RunFunc func(image string, res ResourceSpec, name string) (instanceID string, err error)
