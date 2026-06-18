package vpcmover

// Note: ValidationError and ValidationWarning structs are defined in models.go.
// The ValidationCode constants are also defined there.
// This file adds sentinel vars for each code so handlers can compare or wrap them.

var (
	// Hard blocking errors.
	ErrSourceNotFound       = ValidationError{Code: CodeSourceNotFound, Message: "source VPC not found"}
	ErrDestRegionNotFound   = ValidationError{Code: CodeDestRegionNotFound, Message: "destination region not found"}
	ErrDestZoneNotFound     = ValidationError{Code: CodeDestZoneNotFound, Message: "destination zone not found"}
	ErrAccountMismatch      = ValidationError{Code: CodeAccountMismatch, Message: "cross-account operation requires explicit permission"}
	ErrActiveLockExists     = ValidationError{Code: CodeActiveLock, Message: "active mobility lock exists"}
	ErrCIDRConflict         = ValidationError{Code: CodeCIDRConflict, Message: "CIDR conflict in destination"}
	ErrInsufficientCPU      = ValidationError{Code: CodeInsufficientCPU, Message: "insufficient CPU capacity in destination"}
	ErrInsufficientMemory   = ValidationError{Code: CodeInsufficientMemory, Message: "insufficient memory capacity in destination"}
	ErrInsufficientStorage  = ValidationError{Code: CodeInsufficientStorage, Message: "insufficient storage capacity in destination"}
	ErrGPUUnavailable       = ValidationError{Code: CodeGPUUnavailable, Message: "GPU unavailable or insufficient in destination"}
	ErrInstanceTypeUnavail  = ValidationError{Code: CodeInstanceTypeUnavailable, Message: "instance type unavailable in destination"}
	ErrSnapshotFailed       = ValidationError{Code: CodeSnapshotFailed, Message: "snapshot creation pre-check failed"}
	ErrEncryptionKeyUnavail = ValidationError{Code: CodeEncryptionKeyMissing, Message: "encryption key unavailable in destination"}
	ErrDeleteBlocked        = ValidationError{Code: CodeDeleteBlocked, Message: "delete blocked by attached resources"}
	ErrComplianceLock       = ValidationError{Code: CodeRetentionLock, Message: "compliance retention lock prevents delete"}

	// Non-blocking warnings.
	WarnPublicIPNotPortable    = ValidationWarning{Code: CodePublicIPNotPortable, Message: "public IP not portable", Impact: "New public IPs will be assigned"}
	WarnPrivateDNSConflict     = ValidationWarning{Code: CodePrivateDNSConflict, Message: "private DNS zone conflict", Impact: "DNS zone will be recreated in destination"}
	WarnCertNotPortable        = ValidationWarning{Code: CodeCertNotPortable, Message: "certificate not portable across realms", Impact: "Certificate must be re-issued after cutover"}
	WarnTagPropagationFailed   = ValidationWarning{Code: CodeTagPropagationFailed, Message: "some resource tags may not propagate", Impact: "Manual tag reapplication may be required"}

	// Additional hard errors.
	ErrSecurityGroupConflict = ValidationError{Code: CodeSecurityGroupConflict, Message: "security group rules conflict in destination"}
	ErrNATGatewayUnavail     = ValidationError{Code: CodeNATGatewayUnavail, Message: "NAT gateway unavailable in destination"}
	ErrIGWLimitExceeded      = ValidationError{Code: CodeIGWLimitExceeded, Message: "internet gateway limit exceeded in destination"}
	ErrRouteTableConflict    = ValidationError{Code: CodeRouteTableConflict, Message: "route table conflict in destination"}
	ErrLBTargetUnavail       = ValidationError{Code: CodeLBTargetUnavail, Message: "load balancer target unavailable in destination"}
	ErrKMSKeyUnavail         = ValidationError{Code: CodeKMSKeyUnavail, Message: "KMS key unavailable in destination"}
	ErrPeeringUnsupported    = ValidationError{Code: CodePeeringUnsupported, Message: "VPC peering not supported across target realm"}
	ErrQuotaExceeded         = ValidationError{Code: CodeQuotaExceeded, Message: "resource quota exceeded in destination"}
	ErrRollbackWindowExpired = ValidationError{Code: CodeRollbackWindowExpired, Message: "rollback window has expired"}
	ErrCutoverTimeout        = ValidationError{Code: CodeCutoverTimeout, Message: "cutover operation timed out"}
	ErrInstanceNotStopped    = ValidationError{Code: CodeInstanceNotStopped, Message: "instance must be stopped before live migration"}
	ErrVolumeInUse           = ValidationError{Code: CodeVolumeInUse, Message: "volume is attached and cannot be migrated while in use"}
	ErrNetworkPolicyConflict = ValidationError{Code: CodeNetworkPolicyConflict, Message: "network policy conflict in destination"}
	ErrDependencyOrderError  = ValidationError{Code: CodeDependencyOrderError, Message: "resource dependency ordering error"}
	ErrRealmNotFound         = ValidationError{Code: CodeRealmNotFound, Message: "destination realm not found"}
	ErrMobilityDisabled      = ValidationError{Code: CodeMobilityDisabled, Message: "VPC mobility is disabled for this account"}
	ErrPlanExpired           = ValidationError{Code: CodePlanExpired, Message: "mobility plan has expired and must be recreated"}
)
