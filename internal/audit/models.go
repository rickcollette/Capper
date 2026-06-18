package audit

import "time"

type EventType = string

const (
	EventOrgCreated           EventType = "org.created"
	EventOrgDeleted           EventType = "org.deleted"
	EventAccountCreated       EventType = "account.created"
	EventAccountSuspended     EventType = "account.suspended"
	EventAccountReactivated   EventType = "account.reactivated"
	EventAccountDeleted       EventType = "account.deleted"
	EventIAMUserCreated       EventType = "iam.user.created"
	EventIAMUserDeleted       EventType = "iam.user.deleted"
	EventIAMRoleCreated       EventType = "iam.role.created"
	EventIAMRoleAssumed       EventType = "iam.role.assumed"
	EventIAMPolicyAttached    EventType = "iam.policy.attached"
	EventIAMTokenIssued       EventType = "iam.token.issued"
	EventNodeApproved         EventType = "node.approved"
	EventNodeJoined           EventType = "node.joined"
	EventNodeDeleted          EventType = "node.deleted"
	EventAuthDenied           EventType = "auth.denied"
	EventCertRequested        EventType = "certificate.requested"
	EventCertIssued           EventType = "certificate.issued"
	EventCertAttached         EventType = "certificate.attached"
	EventCertRenewalCompleted EventType = "certificate.renewal.completed"
	EventCertRenewalFailed    EventType = "certificate.renewal.failed"
	EventCertRevoked          EventType = "certificate.revoked"
	EventCertDeleted          EventType = "certificate.deleted"
	EventCertImported         EventType = "certificate.imported"
	EventCertPrivKeyExported  EventType = "certificate.private-key.exported"
	EventVPCMobilityPlanCreated           EventType = "vpc.mobility.plan.created"
	EventVPCMobilityPlanApproved          EventType = "vpc.mobility.plan.approved"
	EventVPCMobilityJobStarted            EventType = "vpc.mobility.job.started"
	EventVPCMobilityJobCompleted          EventType = "vpc.mobility.job.completed"
	EventVPCMobilityJobFailed             EventType = "vpc.mobility.job.failed"
	EventVPCMobilityCutover               EventType = "vpc.mobility.cutover.initiated"
	EventVPCMobilityRollback              EventType = "vpc.mobility.rollback.initiated"
	EventVPCMobilityLockAcquired          EventType = "vpc.mobility.lock.acquired"
	EventVPCMobilityLockReleased          EventType = "vpc.mobility.lock.released"
	EventVPCMobilityStepCompleted         EventType = "vpc.mobility.step.completed"
	EventVPCMobilityStepFailed            EventType = "vpc.mobility.step.failed"
	EventVPCMobilityRollbackWindowExpired EventType = "vpc.mobility.rollback_window.expired"
)

// AuditEvent is an enriched audit record with typed timestamp.
// It mirrors Event but uses time.Time for CreatedAt and is used by
// InsertEvent / ListEvents / ListByResource.
type AuditEvent struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"orgId"`
	AccountID    string    `json:"accountId"`
	ProjectID    string    `json:"projectId"`
	ActorType    string    `json:"actorType"`
	ActorID      string    `json:"actorId"`
	ActorURN     string    `json:"actorUrn"`
	Action       string    `json:"action"`
	ResourceCRN  string    `json:"resourceCrn"`
	Decision     string    `json:"decision"` // "success" or "denied"
	SourceIP     string    `json:"sourceIp"`
	UserAgent    string    `json:"userAgent"`
	RequestID    string    `json:"requestId"`
	MetadataJSON string    `json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}
