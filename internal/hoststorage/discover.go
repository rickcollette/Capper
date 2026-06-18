package hoststorage

import (
	"encoding/json"
	"os/exec"
	"syscall"
)

// lsblkNode mirrors the JSON emitted by `lsblk -J -b`.
type lsblkNode struct {
	Name       string      `json:"name"`
	Path       string      `json:"path"`
	Size       int64       `json:"size"`
	Type       string      `json:"type"`
	Rota       bool        `json:"rota"`
	RM         bool        `json:"rm"`
	Model      string      `json:"model"`
	Serial     string      `json:"serial"`
	FSType     string      `json:"fstype"`
	Mountpoint string      `json:"mountpoint"`
	Children   []lsblkNode `json:"children"`
}

type lsblkOutput struct {
	Blockdevices []lsblkNode `json:"blockdevices"`
}

// Discover enumerates whole disks on the host and classifies each against the
// set of registered pools. It is read-only — it never writes to any device.
func Discover(pools []StoragePool) ([]Disk, error) {
	out, err := exec.Command("lsblk", "-J", "-b", "-o",
		"NAME,PATH,SIZE,TYPE,ROTA,RM,MODEL,SERIAL,FSTYPE,MOUNTPOINT").Output()
	if err != nil {
		return nil, err
	}
	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	poolByMount := map[string]bool{}
	poolByDevice := map[string]bool{}
	for _, p := range pools {
		if p.Mountpoint != "" {
			poolByMount[p.Mountpoint] = true
		}
		if p.Device != "" {
			poolByDevice[p.Device] = true
		}
	}

	var disks []Disk
	for _, n := range parsed.Blockdevices {
		if n.Type != "disk" {
			continue
		}
		d := Disk{
			Name:       n.Name,
			Path:       n.Path,
			SizeBytes:  n.Size,
			Type:       n.Type,
			Rotational: n.Rota,
			Removable:  n.RM,
			Model:      trimSpace(n.Model),
			Serial:     trimSpace(n.Serial),
			FSType:     n.FSType,
			Mountpoint: n.Mountpoint,
		}
		d.State = classify(n, poolByMount, poolByDevice)
		disks = append(disks, d)
	}
	return disks, nil
}

// classify determines a disk's allocation state. A disk that (or whose children)
// backs a registered pool is a pool-member; a disk with any mounted filesystem
// is in-use-by-host; a disk with no filesystem, no mount, and no partitions
// anywhere in its tree is unallocated (safe to register).
func classify(n lsblkNode, poolByMount, poolByDevice map[string]bool) string {
	if poolByDevice[n.Path] || mountsAnyPool(n, poolByMount) {
		return DiskPoolMember
	}
	if hasMount(n) {
		return DiskInUseByHost
	}
	if hasFilesystemOrParts(n) {
		// Formatted or partitioned but unmounted — operator-managed; do not
		// auto-claim it.
		return DiskInUseByHost
	}
	return DiskUnallocated
}

func mountsAnyPool(n lsblkNode, poolByMount map[string]bool) bool {
	if n.Mountpoint != "" && poolByMount[n.Mountpoint] {
		return true
	}
	for _, c := range n.Children {
		if mountsAnyPool(c, poolByMount) {
			return true
		}
	}
	return false
}

func hasMount(n lsblkNode) bool {
	if n.Mountpoint != "" {
		return true
	}
	for _, c := range n.Children {
		if hasMount(c) {
			return true
		}
	}
	return false
}

func hasFilesystemOrParts(n lsblkNode) bool {
	if len(n.Children) > 0 {
		return true
	}
	if n.FSType != "" {
		return true
	}
	return false
}

// statfsCapacity returns the total byte capacity of the filesystem at path.
func statfsCapacity(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Blocks) * st.Bsize, nil
}

func trimSpace(s string) string {
	// lsblk pads model/serial; trim trailing/leading whitespace without pulling strings for one call site elsewhere.
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
