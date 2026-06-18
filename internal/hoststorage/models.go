// Package hoststorage manages the host's physical disks as allocatable capacity.
// It discovers block devices that are physically present, lets an admin register
// the unallocated ones as named storage pools (over an already-mounted path —
// Capper never formats or partitions a disk), and tracks capacity allocations
// drawn from those pools for instance disks and volumes.
package hoststorage

// Disk is a block device discovered on the host.
type Disk struct {
	Name       string `json:"name"`       // kernel name, e.g. "sdb"
	Path       string `json:"path"`       // device path, e.g. "/dev/sdb"
	SizeBytes  int64  `json:"sizeBytes"`
	Type       string `json:"type"`       // "disk", "part", "lvm", …
	Rotational bool   `json:"rotational"` // true = HDD, false = SSD/NVMe
	Removable  bool   `json:"removable"`
	Model      string `json:"model,omitempty"`
	Serial     string `json:"serial,omitempty"`
	FSType     string `json:"fsType,omitempty"`
	Mountpoint string `json:"mountpoint,omitempty"`
	// State is the derived allocation status.
	State string `json:"state"`
}

// Disk states.
const (
	// DiskUnallocated: a whole disk with no filesystem, no mountpoint, and no
	// mounted child partitions — safe to register as a pool.
	DiskUnallocated = "unallocated"
	// DiskPoolMember: the disk (or its mountpoint) backs a registered pool.
	DiskPoolMember = "pool-member"
	// DiskInUseByHost: mounted by the host (root/boot/data) — never auto-touched.
	DiskInUseByHost = "in-use-by-host"
)

// Pool backend types.
const (
	// BackendDirectory: capacity is carved as subdirectories under an
	// already-mounted path. Capper never formats the underlying disk.
	BackendDirectory = "directory"
	// BackendLVM: capacity is carved as logical volumes from an LVM volume
	// group; each allocation is its own ext4-formatted block device.
	BackendLVM = "lvm"
)

// Pool health states (computed by the reconciler).
const (
	PoolHealthy  = "healthy"
	PoolDegraded = "degraded" // mountpoint missing / VG missing / unreadable
)

// StoragePool is a named capacity pool drawn from real host storage. With the
// directory backend, allocations are subdirectories under an already-mounted
// path; with the LVM backend, allocations are logical volumes in a volume group.
type StoragePool struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Backend    string `json:"backend"`          // "directory" | "lvm"
	Mountpoint string `json:"mountpoint,omitempty"` // directory backend: host path
	Device     string `json:"device,omitempty"`     // backing device path, for display
	VGName     string `json:"vgName,omitempty"`      // lvm backend: volume group name
	TotalBytes int64  `json:"totalBytes"`
	Health     string `json:"health,omitempty"`
	CreatedAt  string `json:"createdAt"`

	// Populated on read; not stored.
	AllocatedBytes int64 `json:"allocatedBytes"`
	AvailableBytes int64 `json:"availableBytes"`
}

// Allocation is a capacity claim against a pool, owned by an instance or volume.
// For the directory backend, Path is a subdirectory under the pool mountpoint;
// for the LVM backend, Device is the logical-volume block device path.
type Allocation struct {
	ID        string `json:"id"`
	PoolID    string `json:"poolId"`
	Owner     string `json:"owner"`     // e.g. instance ID or volume ID
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`   // directory backend: subdirectory
	Device    string `json:"device,omitempty"` // lvm backend: /dev/<vg>/<lv>
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}
