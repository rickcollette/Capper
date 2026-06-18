package csd

import "time"

const (
	ModeSharedFS     = "shared-fs"
	ModeSingleWriter = "single-writer"
	ModeSharedBlock  = "shared-block"

	StatusCreating  = "creating"
	StatusAvailable = "available"
	StatusAttaching = "attaching"
	StatusAttached  = "attached"
	StatusDegraded  = "degraded"
	StatusRepairing = "repairing"
	StatusReadonly  = "readonly"
	StatusFailed    = "failed"
	StatusDeleting  = "deleting"
	StatusDeleted   = "deleted"

	AccessRO = "ro"
	AccessRW = "rw"

	InodeFile    = "file"
	InodeDir     = "directory"
	InodeSymlink = "symlink"

	LeaseRead       = "read"
	LeaseWrite      = "write"
	LeaseMeta       = "metadata"
	LeaseExclusive  = "exclusive"
	LeaseDelegation = "delegation"

	JournalCreate   = "create"
	JournalMkdir    = "mkdir"
	JournalUnlink   = "unlink"
	JournalRename   = "rename"
	JournalTruncate = "truncate"
	JournalWrite    = "write"
	JournalChmod    = "chmod"
	JournalChown    = "chown"
	JournalFsync    = "fsync"
	JournalSnapshot = "snapshot"

	ReplicaPrimary   = "primary"
	ReplicaSecondary = "secondary"
	ReplicaWitness   = "witness"

	ExtentSize     = 4 * 1024 * 1024 // 4 MiB
	InlineMaxBytes = 16 * 1024        // 16 KiB — stored inline in inode row
	LeaseTTL       = 15 * time.Second
)

type Volume struct {
	ID              string `json:"id"`
	Project         string `json:"project"`
	Name            string `json:"name"`
	Mode            string `json:"mode"`
	SizeBytes       int64  `json:"sizeBytes"`
	UsedBytes       int64  `json:"usedBytes"`
	Status          string `json:"status"`
	RealmID         string `json:"realmId,omitempty"`
	RegionID        string `json:"regionId,omitempty"`
	ZoneID          string `json:"zoneId,omitempty"`
	StorageClass    string `json:"storageClass"`
	ReplicaCount    int    `json:"replicaCount"`
	PrimaryNodeID   string `json:"primaryNodeId,omitempty"`
	Epoch           int64  `json:"epoch"`
	Encrypted       bool   `json:"encrypted"`
	EncryptionKeyID string `json:"encryptionKeyId,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type Attachment struct {
	ID         string `json:"id"`
	VolumeID   string `json:"volumeId"`
	InstanceID string `json:"instanceId"`
	NodeID     string `json:"nodeId,omitempty"`
	MountPath  string `json:"mountPath"`
	AccessMode string `json:"accessMode"`
	ClientID   string `json:"clientId,omitempty"`
	LeaseEpoch int64  `json:"leaseEpoch"`
	Status     string `json:"status"`
	AttachedAt string `json:"attachedAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type Inode struct {
	ID         string `json:"id"`
	VolumeID   string `json:"volumeId"`
	ParentID   string `json:"parentId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	SizeBytes  int64  `json:"sizeBytes"`
	ModeBits   uint32 `json:"modeBits"`
	UID        uint32 `json:"uid"`
	GID        uint32 `json:"gid"`
	LinkCount  int    `json:"linkCount"`
	Version    int64  `json:"version"`
	ExtentRoot string `json:"extentRoot,omitempty"`
	InlineData []byte `json:"-"`
	CreatedAt  string `json:"createdAt"`
	ModifiedAt string `json:"modifiedAt"`
	AccessedAt string `json:"accessedAt"`
}

type Extent struct {
	ID          string `json:"id"`
	VolumeID    string `json:"volumeId"`
	InodeID     string `json:"inodeId"`
	OffsetBytes int64  `json:"offsetBytes"`
	LengthBytes int64  `json:"lengthBytes"`
	ObjectKey   string `json:"objectKey"`
	Checksum    string `json:"checksum"`
	RefCount    int    `json:"refCount"`
	CreatedAt   string `json:"createdAt"`
}

type Lease struct {
	ID         string    `json:"id"`
	VolumeID   string    `json:"volumeId"`
	InodeID    string    `json:"inodeId"`
	ClientID   string    `json:"clientId"`
	SessionID  string    `json:"sessionId"`
	LeaseType  string    `json:"leaseType"`
	RangeStart int64     `json:"rangeStart"`
	RangeEnd   int64     `json:"rangeEnd"`
	Epoch      int64     `json:"epoch"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  string    `json:"createdAt"`
}

type JournalEntry struct {
	ID          string         `json:"id"`
	VolumeID    string         `json:"volumeId"`
	Seq         int64          `json:"seq"`
	ClientID    string         `json:"clientId"`
	SessionID   string         `json:"sessionId"`
	Operation   string         `json:"operation"`
	InodeID     string         `json:"inodeId"`
	Payload     map[string]any `json:"payload"`
	Checksum    string         `json:"checksum"`
	Status      string         `json:"status"`
	CreatedAt   string         `json:"createdAt"`
	CommittedAt string         `json:"committedAt,omitempty"`
}

type Replica struct {
	ID          string `json:"id"`
	VolumeID    string `json:"volumeId"`
	Role        string `json:"role"`
	RealmID     string `json:"realmId,omitempty"`
	RegionID    string `json:"regionId,omitempty"`
	ZoneID      string `json:"zoneId,omitempty"`
	NodeID      string `json:"nodeId"`
	Addr        string `json:"addr,omitempty"` // host:port for CSDP replication
	BackendType string `json:"backendType"`
	BackendPath string `json:"backendPath"`
	Status      string `json:"status"`
	LagBytes    int64  `json:"lagBytes"`
	LastSeq     int64  `json:"lastSeq"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// VolumeReplica is an alias kept for replication package compatibility.
type VolumeReplica = Replica

type Snapshot struct {
	ID          string `json:"id"`
	VolumeID    string `json:"volumeId"`
	Name        string `json:"name"`
	RootVersion int64  `json:"rootVersion"`
	Status      string `json:"status"`
	Consistent  bool   `json:"consistent"`
	SizeBytes   int64  `json:"sizeBytes"`
	CreatedAt   string `json:"createdAt"`
}
