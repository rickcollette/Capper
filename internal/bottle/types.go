package bottle

// BottleStatus is the lifecycle state of a stored bottle definition.
type BottleStatus string

const (
	BottleActive   BottleStatus = "active"
	BottleArchived BottleStatus = "archived"
)

// DeploymentStatus is the lifecycle state of a bottle deployment.
type DeploymentStatus string

const (
	DeploymentPlanning  DeploymentStatus = "planning"
	DeploymentPlanned   DeploymentStatus = "planned"
	DeploymentBuilding  DeploymentStatus = "building"
	DeploymentDeploying DeploymentStatus = "deploying"
	DeploymentRunning   DeploymentStatus = "running"
	DeploymentDegraded  DeploymentStatus = "degraded"
	DeploymentFailed    DeploymentStatus = "failed"
	DeploymentUpgrading DeploymentStatus = "upgrading"
	DeploymentRemoving  DeploymentStatus = "removing"
	DeploymentRemoved   DeploymentStatus = "removed"
)

// JobType describes the kind of operation a bottle job performs.
type JobType string

const (
	JobTypeValidate JobType = "validate"
	JobTypePlan     JobType = "plan"
	JobTypeBuild    JobType = "build"
	JobTypeDeploy   JobType = "deploy"
	JobTypeUpgrade  JobType = "upgrade"
	JobTypeRemove   JobType = "remove"
)

// JobStatus tracks execution of a bottle job.
type JobStatus string

const (
	JobStatusQueued  JobStatus = "queued"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
)

// Bottle is the stored record for an imported bottle definition.
type Bottle struct {
	ID          string       `json:"id"`
	Project     string       `json:"project"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Author      string       `json:"author"`
	License     string       `json:"license"`
	Source      string       `json:"source"`
	Digest      string       `json:"digest"`
	RawJSON     string       `json:"rawJson,omitempty"`
	Status      BottleStatus `json:"status"`
	Tags        []string     `json:"tags"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
}

// BottleDeployment tracks a live install of a bottle.
type BottleDeployment struct {
	ID         string           `json:"id"`
	Project    string           `json:"project"`
	BottleID   string           `json:"bottleId"`
	Name       string           `json:"name"`
	Version    string           `json:"version"`
	Status     DeploymentStatus `json:"status"`
	Parameters map[string]string `json:"parameters"`
	Outputs    map[string]string `json:"outputs"`
	Resources  []DeployedResource `json:"resources"`
	CreatedAt  string           `json:"createdAt"`
	UpdatedAt  string           `json:"updatedAt"`
}

// DeployedResource is a Capper resource that was created by a deployment.
type DeployedResource struct {
	Kind string `json:"kind"` // "network", "instance", "lb", "dns", "secret", "volume"
	Name string `json:"name"`
	ID   string `json:"id"`
}

// BottleJob is a record of an async operation on a bottle or deployment.
type BottleJob struct {
	ID           string    `json:"id"`
	Project      string    `json:"project"`
	DeploymentID string    `json:"deploymentId"`
	BottleID     string    `json:"bottleId"`
	JobType      JobType   `json:"jobType"`
	Status       JobStatus `json:"status"`
	Logs         string    `json:"logs,omitempty"`
	ResultJSON   string    `json:"resultJson,omitempty"`
	CreatedAt    string    `json:"createdAt"`
	UpdatedAt    string    `json:"updatedAt"`
}

// ---- Spec types (parsed from the bottle JSON document) ---------------------

// BottleSpec is the top-level parsed bottle document.
type BottleSpec struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   BottleMetadata   `json:"metadata"`
	Spec       BottleSpecBody   `json:"spec"`
}

// BottleMetadata holds the bottle's identity fields.
type BottleMetadata struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	License     string   `json:"license"`
	Homepage    string   `json:"homepage"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags"`
}

// BottleSpecBody is the spec section of a bottle document.
type BottleSpecBody struct {
	BaseCapsule  string                     `json:"baseCapsule"`
	Parameters   map[string]ParameterSpec   `json:"parameters"`
	Build        *BuildSpec                 `json:"build"`
	Resources    ResourcesSpec              `json:"resources"`
	Services     []ServiceSpec              `json:"services"`
	Outputs      map[string]OutputSpec      `json:"outputs"`
	Dependencies []DependencySpec           `json:"dependencies"`
	Capabilities CapabilitiesSpec           `json:"capabilities"`
}

// ParameterSpec describes a user-configurable parameter.
type ParameterSpec struct {
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Min         int    `json:"min,omitempty"`
	Max         int    `json:"max,omitempty"`
}

// BuildSpec describes how to build a capsule from a base image.
type BuildSpec struct {
	Enabled     bool        `json:"enabled"`
	OutputImage string      `json:"outputImage"`
	Version     string      `json:"version"`
	Steps       []BuildStep `json:"steps"`
	Files       []BuildFile `json:"files"`
	Entrypoint  []string    `json:"entrypoint"`
	Args        []string    `json:"args"`
}

// BuildStep is one step in a bottle build.
type BuildStep struct {
	Run      string        `json:"run,omitempty"`
	Copy     *CopyStep     `json:"copy,omitempty"`
	Download *DownloadStep `json:"download,omitempty"`
}

// CopyStep copies a local file into the build rootfs.
type CopyStep struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
}

// DownloadStep downloads a remote file into the build rootfs.
type DownloadStep struct {
	URL      string `json:"url"`
	Dest     string `json:"dest"`
	Checksum string `json:"checksum"`
}

// BuildFile injects a file with inline content into the build rootfs.
type BuildFile struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
}

// ResourcesSpec describes the infrastructure resources a bottle needs.
type ResourcesSpec struct {
	Networks      []NetworkSpec      `json:"networks"`
	Volumes       []VolumeSpec       `json:"volumes"`
	Secrets       []SecretSpec       `json:"secrets"`
	LoadBalancers []LoadBalancerSpec  `json:"loadBalancers"`
	DNS           []DNSSpec          `json:"dns"`
}

// NetworkSpec declares a network resource.
type NetworkSpec struct {
	Name   string `json:"name"`
	Mode   string `json:"mode"`   // "nat", "bridge"
	Subnet string `json:"subnet"` // "auto" or CIDR
}

// VolumeSpec declares a volume resource.
type VolumeSpec struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // "csd", "local"
	Mode      string `json:"mode"`       // "shared-fs", "single-writer"
	Size      string `json:"size"`       // "50G", "1T"
	MountPath string `json:"mountPath"`
	Retain    bool   `json:"retain"`
}

// SecretSpec declares a secret that will be created or referenced.
type SecretSpec struct {
	Name               string         `json:"name"`
	ValueFromParameter string         `json:"valueFromParameter,omitempty"`
	Generate           *SecretGenSpec `json:"generate,omitempty"`
}

// SecretGenSpec describes auto-generated secret values.
type SecretGenSpec struct {
	Type   string `json:"type"`   // "password", "token"
	Length int    `json:"length"`
}

// LoadBalancerSpec declares an LB resource.
type LoadBalancerSpec struct {
	Name          string `json:"name"`
	Mode          string `json:"mode"`          // "tcp", "http"
	ListenPort    string `json:"listenPort"`
	TargetService string `json:"targetService"`
	TargetPort    string `json:"targetPort"`
}

// DNSSpec declares a DNS record resource.
type DNSSpec struct {
	Name   string `json:"name"`
	Host   string `json:"host,omitempty"`
	Type   string `json:"type,omitempty"`   // "A", "CNAME"
	Target string `json:"target,omitempty"` // LB name or IP
}

// ServiceSpec describes one runnable service in the bottle.
type ServiceSpec struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Replicas     string            `json:"replicas"` // may be a template expression
	Network      string            `json:"network"`
	InstanceType string            `json:"instanceType"`
	Resources    ServiceResources  `json:"resources"`
	Env          map[string]string `json:"env"`
	Secrets      map[string]SecretRef `json:"secrets"`
	Volumes      []VolumeMountSpec `json:"volumes"`
	Ports        []PortSpec        `json:"ports"`
	Health       *HealthSpec       `json:"health"`
	RestartPolicy string           `json:"restartPolicy"`
	Autoscaling  *AutoscalingSpec  `json:"autoscaling"`
	Placement    *PlacementSpec    `json:"placement"`
}

// ServiceResources declares CPU/memory for a service.
type ServiceResources struct {
	CPU    int    `json:"cpu"`
	Memory string `json:"memory"`
}

// SecretRef binds an env var to a secret.
type SecretRef struct {
	FromSecret string `json:"fromSecret"`
}

// VolumeMountSpec mounts a named volume into a service.
type VolumeMountSpec struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	Access    string `json:"access"` // "rw", "ro"
}

// PortSpec declares an exposed port on a service.
type PortSpec struct {
	Name     string `json:"name"`
	Port     string `json:"port"` // may be a template expression
	Protocol string `json:"protocol"` // "tcp", "udp"
}

// HealthSpec configures health checking for a service.
type HealthSpec struct {
	Type             string `json:"type"` // "tcp", "http"
	Path             string `json:"path,omitempty"`
	Port             string `json:"port"`
	IntervalSeconds  int    `json:"intervalSeconds"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	FailureThreshold int    `json:"failureThreshold"`
}

// AutoscalingSpec configures horizontal autoscaling for a service.
type AutoscalingSpec struct {
	Enabled     bool   `json:"enabled"`
	MinReplicas int    `json:"minReplicas"`
	MaxReplicas int    `json:"maxReplicas"`
	Metric      string `json:"metric"` // "cpu", "memory"
	Target      int    `json:"target"` // percentage
}

// PlacementSpec constrains where a service runs.
type PlacementSpec struct {
	Region   string            `json:"region"`
	Strategy string            `json:"strategy"`
	MinZones int               `json:"minZones"`
	Labels   map[string]string `json:"labels"`
}

// OutputSpec declares an output value from a deployment.
type OutputSpec struct {
	Description string `json:"description"`
	Value       string `json:"value"` // may be a template expression
}

// DependencySpec declares another bottle this bottle depends on.
type DependencySpec struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Alias    string `json:"alias"`
	Optional bool   `json:"optional"`
}

// CapabilitiesSpec declares privileged/unusual requirements.
type CapabilitiesSpec struct {
	RequiresPrivileged       bool `json:"requiresPrivileged"`
	RequiresHostNetwork      bool `json:"requiresHostNetwork"`
	RequiresHostMounts       bool `json:"requiresHostMounts"`
	RequiresExternalDownloads bool `json:"requiresExternalDownloads"`
	RequiresSecrets          bool `json:"requiresSecrets"`
}

// PlanAction describes one resource operation in a bottle plan.
type PlanAction struct {
	Action  string `json:"action"`  // "create", "update", "no-op", "warn", "block"
	Kind    string `json:"kind"`    // "network", "volume", "secret", "instance", "lb", "dns"
	Name    string `json:"name"`
	Detail  string `json:"detail"`
}
