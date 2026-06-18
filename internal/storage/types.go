package storage

// Volume is a directory-backed block storage volume that can be attached to
// an instance as a bind mount.
type Volume struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	SizeBytes          int64  `json:"sizeBytes"`
	Class              string `json:"class"`
	Backend            string `json:"backend"`
	Path               string `json:"path"`
	Encrypted          bool   `json:"encrypted"`
	AttachedInstanceID string `json:"attachedInstanceId,omitempty"`
	AttachedPath       string `json:"attachedPath,omitempty"`
	CreatedAt          string `json:"createdAt"`
}

const (
	VolumeBackendDirectory = "directory"

	VolumeClassLocalSSD = "local-ssd"
	VolumeClassLocal    = "local"
)

// Bucket is a local-filesystem-backed object storage bucket.
type Bucket struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Backend    string `json:"backend"`
	Path       string `json:"path"`
	Versioning bool   `json:"versioning"`
	Encrypted  bool   `json:"encrypted"`
	KMSKeyName string `json:"kmsKeyName,omitempty"` // KMS key for at-rest object encryption
	QuotaBytes int64  `json:"quotaBytes,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

const (
	BucketBackendLocal = "local"
)

// Object is a file stored inside a bucket, returned by object CRUD operations.
// Metadata comes from xl.meta on disk; not stored in SQLite.
type Object struct {
	Key         string `json:"key"`
	BucketID    string `json:"bucketId"`
	BucketName  string `json:"bucketName,omitempty"`
	SizeBytes   int64  `json:"sizeBytes"`
	Digest      string `json:"digest"` // sha256 hex (ETag)
	ContentType string `json:"contentType,omitempty"`
	CreatedAt   string `json:"createdAt"` // derived from xl.meta lastModified
}

// Snapshot is a point-in-time tar.zst capture of a volume's directory.
type Snapshot struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	Path       string `json:"path"`
	Digest     string `json:"digest"`
	SizeBytes  int64  `json:"sizeBytes"`
	CreatedAt  string `json:"createdAt"`
}

const (
	SnapshotSourceVolume = "volume"
	SnapshotSourceBucket = "bucket"
)

// AttachOptions carries parameters for volume attachment.
type AttachOptions struct {
	InstanceID   string
	InstancePath string // path inside the instance rootfs
}

// CreateVolumeOptions carries all parameters for volume creation.
type CreateVolumeOptions struct {
	Name      string
	SizeBytes int64
	Class     string
	Encrypted bool
}

// CreateBucketOptions carries all parameters for bucket creation.
type CreateBucketOptions struct {
	Name       string
	Versioning bool
	Encrypted  bool
	QuotaBytes int64
	KMSKeyName string // KMS key for at-rest object encryption
}
