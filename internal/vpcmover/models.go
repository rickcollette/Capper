// Package vpcmover implements VPC mobility: copy, move, delete, and failover
// of Capper VPCs with full dependency graph awareness.
//
// Operations are modelled as a two-phase protocol:
//  1. Plan — inventory the source VPC, validate destination compatibility,
//     and generate an immutable ordered step list.
//  2. Execute — run each step, tracking state in vpc_mobility_jobs and
//     vpc_mobility_steps. Cutover and rollback are separate steps.
//
// A VPC is never directly mutated; move = copy + validate + cutover + retire.
package vpcmover

// Operation is the type of mobility action.
type Operation string

const (
	OperationCopy     Operation = "copy"
	OperationMove     Operation = "move"
	OperationDelete   Operation = "delete"
	OperationFailover Operation = "failover"
	OperationRelocate Operation = "relocate"
)

// CopyMode controls which resource categories are included in a copy.
type CopyMode string

const (
	CopyModeTopologyOnly            CopyMode = "topology-only"
	CopyModeTopologyAndCompute      CopyMode = "topology-and-compute"
	CopyModeTopologyComputeStorage  CopyMode = "topology-compute-and-storage"
	CopyModeFull                    CopyMode = "full"
)

// Strategy is the cutover approach for a move or failover.
type Strategy string

const (
	StrategyPlannedCutover Strategy = "planned-cutover"
	StrategyHotStandby     Strategy = "hot-standby"
	StrategyColdRestore    Strategy = "cold-restore"
	StrategyEmergencyRestore Strategy = "emergency-restore"
	StrategyBlueGreen      Strategy = "blue-green"
)

// Plan status values.
const (
	PlanStatusDraft      = "draft"
	PlanStatusValidated  = "validated"
	PlanStatusBlocked    = "blocked"
	PlanStatusApproved   = "approved"
	PlanStatusExecuting  = "executing"
	PlanStatusCompleted  = "completed"
	PlanStatusFailed     = "failed"
	PlanStatusCancelled  = "cancelled"
	PlanStatusExpired    = "expired"
)

// Job status values.
const (
	JobStatusQueued             = "queued"
	JobStatusRunning            = "running"
	JobStatusWaitingApproval    = "waiting_for_approval"
	JobStatusWaitingCutover     = "waiting_for_cutover"
	JobStatusCompleted          = "completed"
	JobStatusFailed             = "failed"
	JobStatusRollingBack        = "rolling_back"
	JobStatusRolledBack         = "rolled_back"
	JobStatusCancelled          = "cancelled"
)

// Step status values.
const (
	StepStatusPending    = "pending"
	StepStatusRunning    = "running"
	StepStatusCompleted  = "completed"
	StepStatusFailed     = "failed"
	StepStatusSkipped    = "skipped"
	StepStatusRetrying   = "retrying"
	StepStatusRolledBack = "rolled_back"
)

// Lock type values.
const (
	LockTypeMobilityPlan = "mobility-plan"
	LockTypeCopy         = "copy"
	LockTypeMove         = "move"
	LockTypeDelete       = "delete"
	LockTypeCutover      = "cutover"
	LockTypeFreeze       = "freeze"
)

// ValidationCode is a stable code for compatibility check failures.
type ValidationCode string

const (
	CodeSourceNotFound         ValidationCode = "VPCM-001"
	CodeDestRegionNotFound     ValidationCode = "VPCM-002"
	CodeDestZoneNotFound       ValidationCode = "VPCM-003"
	CodeAccountMismatch        ValidationCode = "VPCM-004"
	CodeActiveLock             ValidationCode = "VPCM-010"
	CodeCIDRConflict           ValidationCode = "VPCM-020"
	CodeInsufficientCPU        ValidationCode = "VPCM-030"
	CodeInsufficientMemory     ValidationCode = "VPCM-031"
	CodeInsufficientStorage    ValidationCode = "VPCM-032"
	CodeGPUUnavailable         ValidationCode = "VPCM-033"
	CodeInstanceTypeUnavailable ValidationCode = "VPCM-040"
	CodeSnapshotFailed         ValidationCode = "VPCM-050"
	CodeEncryptionKeyMissing   ValidationCode = "VPCM-060"
	CodePublicIPNotPortable    ValidationCode = "VPCM-070"
	CodePrivateDNSConflict     ValidationCode = "VPCM-080"
	CodeSecurityGroupConflict  ValidationCode = "VPCM-090"
	CodeNATGatewayUnavail      ValidationCode = "VPCM-100"
	CodeIGWLimitExceeded       ValidationCode = "VPCM-110"
	CodeRouteTableConflict     ValidationCode = "VPCM-120"
	CodeLBTargetUnavail        ValidationCode = "VPCM-130"
	CodeCertNotPortable        ValidationCode = "VPCM-140"
	CodeKMSKeyUnavail          ValidationCode = "VPCM-150"
	CodePeeringUnsupported     ValidationCode = "VPCM-160"
	CodeDeleteBlocked          ValidationCode = "VPCM-170"
	CodeRetentionLock          ValidationCode = "VPCM-180"
	CodeQuotaExceeded          ValidationCode = "VPCM-190"
	CodeRollbackWindowExpired  ValidationCode = "VPCM-200"
	CodeCutoverTimeout         ValidationCode = "VPCM-210"
	CodeInstanceNotStopped     ValidationCode = "VPCM-220"
	CodeVolumeInUse            ValidationCode = "VPCM-230"
	CodeNetworkPolicyConflict  ValidationCode = "VPCM-240"
	CodeTagPropagationFailed   ValidationCode = "VPCM-250"
	CodeDependencyOrderError   ValidationCode = "VPCM-260"
	CodeRealmNotFound          ValidationCode = "VPCM-270"
	CodeMobilityDisabled       ValidationCode = "VPCM-280"
	CodePlanExpired            ValidationCode = "VPCM-290"
)

// MobilityPlan is an immutable execution document generated from inventory.
type MobilityPlan struct {
	ID                 string    `json:"id"`
	OrgID              string    `json:"orgId"`
	AccountID          string    `json:"accountId"`
	ProjectID          string    `json:"projectId"`
	SourceVPCID        string    `json:"sourceVpcId"`
	DestinationVPCID   string    `json:"destinationVpcId"`
	Operation          Operation `json:"operation"`
	Strategy           string    `json:"strategy"`
	TargetRealmID      string    `json:"targetRealmId"`
	TargetRegionID     string    `json:"targetRegionId"`
	TargetZoneID       string    `json:"targetZoneId"`
	Status             string    `json:"status"`
	IncludeJSON        string    `json:"include"`
	ExcludeJSON        string    `json:"exclude"`
	OptionsJSON        string    `json:"options"`
	InventoryJSON      string    `json:"inventory"`
	PlanJSON           string    `json:"plan"`
	WarningsJSON       string    `json:"warnings"`
	ErrorsJSON         string    `json:"errors"`
	CreatedBy          string    `json:"createdBy"`
	CreatedAt          string    `json:"createdAt"`
	UpdatedAt          string    `json:"updatedAt"`
}

// MobilityJob tracks execution of a plan.
type MobilityJob struct {
	ID                 string    `json:"id"`
	PlanID             string    `json:"planId"`
	OrgID              string    `json:"orgId"`
	AccountID          string    `json:"accountId"`
	SourceVPCID        string    `json:"sourceVpcId"`
	DestinationVPCID   string    `json:"destinationVpcId"`
	Operation          Operation `json:"operation"`
	Status             string    `json:"status"`
	CurrentStep        string    `json:"currentStep"`
	ProgressPercent    int       `json:"progressPercent"`
	RollbackAvailable  bool      `json:"rollbackAvailable"`
	RollbackExpiresAt  string    `json:"rollbackExpiresAt"`
	StartedAt          string    `json:"startedAt"`
	FinishedAt         string    `json:"finishedAt"`
	ErrorMessage       string    `json:"errorMessage"`
	CreatedBy          string    `json:"createdBy"`
	CreatedAt          string    `json:"createdAt"`
	UpdatedAt          string    `json:"updatedAt"`
}

// MobilityStep is one ordered step within a job.
type MobilityStep struct {
	ID           string `json:"id"`
	JobID        string `json:"jobId"`
	StepOrder    int    `json:"stepOrder"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	InputJSON    string `json:"input"`
	OutputJSON   string `json:"output"`
	ErrorMessage string `json:"errorMessage"`
	RetryCount   int    `json:"retryCount"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// ResourceMapping records the old→new mapping for a resource created by a job.
type ResourceMapping struct {
	ID                  string `json:"id"`
	JobID               string `json:"jobId"`
	OrgID               string `json:"orgId"`
	AccountID           string `json:"accountId"`
	SourceResourceType  string `json:"sourceResourceType"`
	SourceResourceID    string `json:"sourceResourceId"`
	DestResourceType    string `json:"destResourceType"`
	DestResourceID      string `json:"destResourceId"`
	MappingJSON         string `json:"mapping"`
	CreatedAt           string `json:"createdAt"`
}

// VPCLock prevents concurrent mobility operations on the same VPC.
type VPCLock struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	AccountID string `json:"accountId"`
	VPCID     string `json:"vpcId"`
	LockType  string `json:"lockType"`
	Reason    string `json:"reason"`
	JobID     string `json:"jobId"`
	ExpiresAt string `json:"expiresAt"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
}

// ValidationWarning is a non-blocking compatibility warning.
type ValidationWarning struct {
	Code    ValidationCode `json:"code"`
	Message string         `json:"message"`
	Impact  string         `json:"impact"`
}

// ValidationError blocks plan execution.
type ValidationError struct {
	Code    ValidationCode `json:"code"`
	Message string         `json:"message"`
}

// PlanOptions configures how a plan is generated.
type PlanOptions struct {
	CopyMode              CopyMode `json:"copyMode"`
	Strategy              Strategy `json:"strategy"`
	PreserveCIDR          bool     `json:"preserveCidr"`
	CopySnapshots         bool     `json:"copySnapshots"`
	CreateInstancesStopped bool    `json:"createInstancesStopped"`
	DNSTTLSeconds         int      `json:"dnsTtlSeconds"`
	RollbackWindowSeconds int      `json:"rollbackWindowSeconds"`
	IncludeResources      []string `json:"include"`
	ExcludeResources      []string `json:"exclude"`
}

// VPCInventory is the enumerated state of the source VPC.
type VPCInventory struct {
	VPCID         string   `json:"vpcId"`
	SubnetCount   int      `json:"subnetCount"`
	RouteCount    int      `json:"routeCount"`
	FirewallRules int      `json:"firewallRules"`
	InstanceCount int      `json:"instanceCount"`
	VolumeCount   int      `json:"volumeCount"`
	LBCount       int      `json:"lbCount"`
	DNSRecords    int      `json:"dnsRecords"`
	StaticIPs     int      `json:"staticIps"`
	ResourceIDs   []string `json:"resourceIds"`
}
