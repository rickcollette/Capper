package hoststorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Manager orchestrates disk discovery, pool registration, and allocation while
// enforcing capacity limits. It never formats or partitions a device: a pool is
// registered over a path the operator has already mounted.
type Manager struct {
	store *Store
}

// NewManager wraps a Store.
func NewManager(store *Store) *Manager { return &Manager{store: store} }

// Store exposes the underlying store.
func (m *Manager) Store() *Store { return m.store }

// Disks returns the host's disks classified against the registered pools.
func (m *Manager) Disks() ([]Disk, error) {
	pools, err := m.store.ListPools()
	if err != nil {
		return nil, err
	}
	return Discover(pools)
}

// CreatePoolOptions configures pool registration.
type CreatePoolOptions struct {
	Name       string
	Backend    string // "directory" (default) | "lvm"
	Mountpoint string // directory backend
	Device     string // directory backend: backing device, for display
	VGName     string // lvm backend: volume group name
	TotalBytes int64  // 0 => derive from the backend
}

// CreatePool registers a pool. The directory backend draws from an already-mounted
// writable path (Capper never formats the underlying disk). The LVM backend draws
// logical volumes from an existing volume group.
func (m *Manager) CreatePool(opts CreatePoolOptions) (StoragePool, error) {
	if opts.Name == "" {
		return StoragePool{}, fmt.Errorf("hoststorage: pool name is required")
	}
	backend := opts.Backend
	if backend == "" {
		backend = BackendDirectory
	}
	switch backend {
	case BackendDirectory:
		if opts.Mountpoint == "" {
			return StoragePool{}, fmt.Errorf("hoststorage: mountpoint is required")
		}
		info, err := os.Stat(opts.Mountpoint)
		if err != nil {
			return StoragePool{}, fmt.Errorf("hoststorage: mountpoint %s: %w", opts.Mountpoint, err)
		}
		if !info.IsDir() {
			return StoragePool{}, fmt.Errorf("hoststorage: mountpoint %s is not a directory", opts.Mountpoint)
		}
		total := opts.TotalBytes
		if total <= 0 {
			if cap, cerr := statfsCapacity(opts.Mountpoint); cerr == nil {
				total = cap
			}
		}
		return m.store.InsertPool(StoragePool{
			Name: opts.Name, Backend: BackendDirectory, Mountpoint: opts.Mountpoint,
			Device: opts.Device, TotalBytes: total,
		})
	case BackendLVM:
		if opts.VGName == "" {
			return StoragePool{}, fmt.Errorf("hoststorage: vgName is required for an LVM pool")
		}
		if !lvmAvailable() {
			return StoragePool{}, fmt.Errorf("hoststorage: LVM tools are not installed on this host")
		}
		total := opts.TotalBytes
		if total <= 0 {
			n, err := vgSizeBytes(context.Background(), opts.VGName)
			if err != nil {
				return StoragePool{}, err
			}
			total = n
		}
		return m.store.InsertPool(StoragePool{
			Name: opts.Name, Backend: BackendLVM, VGName: opts.VGName, TotalBytes: total,
		})
	default:
		return StoragePool{}, fmt.Errorf("hoststorage: unknown backend %q", backend)
	}
}

// ListPools returns pools with live capacity accounting.
func (m *Manager) ListPools() ([]StoragePool, error) {
	pools, err := m.store.ListPools()
	if err != nil {
		return nil, err
	}
	for i := range pools {
		m.fillUsage(&pools[i])
	}
	return pools, nil
}

// GetPool returns a single pool with capacity accounting.
func (m *Manager) GetPool(idOrName string) (StoragePool, error) {
	p, err := m.store.GetPool(idOrName)
	if err != nil {
		return StoragePool{}, err
	}
	m.fillUsage(&p)
	return p, nil
}

func (m *Manager) fillUsage(p *StoragePool) {
	allocated, _ := m.store.AllocatedBytes(p.ID)
	p.AllocatedBytes = allocated
	p.AvailableBytes = p.TotalBytes - allocated
	if p.AvailableBytes < 0 {
		p.AvailableBytes = 0
	}
}

// DeletePool removes a pool. It refuses if the pool still has allocations.
func (m *Manager) DeletePool(idOrName string) error {
	p, err := m.store.GetPool(idOrName)
	if err != nil {
		return fmt.Errorf("hoststorage: pool %q not found", idOrName)
	}
	n, err := m.store.CountAllocations(p.ID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("hoststorage: pool %q has %d allocation(s); release them first", p.Name, n)
	}
	return m.store.DeletePool(p.ID)
}

// CanAllocate reports whether a pool has room for sizeBytes more.
func (m *Manager) CanAllocate(poolID string, sizeBytes int64) error {
	p, err := m.store.GetPool(poolID)
	if err != nil {
		return fmt.Errorf("hoststorage: pool not found: %s", poolID)
	}
	allocated, err := m.store.AllocatedBytes(p.ID)
	if err != nil {
		return err
	}
	if allocated+sizeBytes > p.TotalBytes {
		return fmt.Errorf("hoststorage: pool %q has insufficient capacity (%d requested, %d available of %d)",
			p.Name, sizeBytes, p.TotalBytes-allocated, p.TotalBytes)
	}
	return nil
}

// AllocateOptions configures a capacity claim.
type AllocateOptions struct {
	PoolID    string
	Owner     string
	Name      string
	SizeBytes int64
}

// Allocate carves capacity from a pool, refusing to overcommit it. For a
// directory pool it creates a subdirectory under the mountpoint; for an LVM pool
// it creates an ext4-formatted logical volume. The recorded Allocation carries
// Path (directory) or Device (LVM) for the caller to back storage onto.
func (m *Manager) Allocate(opts AllocateOptions) (Allocation, error) {
	if opts.Name == "" {
		return Allocation{}, fmt.Errorf("hoststorage: allocation name is required")
	}
	if opts.SizeBytes <= 0 {
		return Allocation{}, fmt.Errorf("hoststorage: allocation size must be positive")
	}
	pool, err := m.store.GetPool(opts.PoolID)
	if err != nil {
		return Allocation{}, fmt.Errorf("hoststorage: pool not found: %s", opts.PoolID)
	}
	if err := m.CanAllocate(pool.ID, opts.SizeBytes); err != nil {
		return Allocation{}, err
	}

	rec := Allocation{PoolID: pool.ID, Owner: opts.Owner, Name: opts.Name, SizeBytes: opts.SizeBytes}
	switch pool.Backend {
	case BackendLVM:
		lv := sanitizeLVName(opts.Name)
		device, lerr := lvCreate(context.Background(), pool.VGName, lv, opts.SizeBytes)
		if lerr != nil {
			return Allocation{}, lerr
		}
		rec.Device = device
	default: // directory
		dir := filepath.Join(pool.Mountpoint, "capper-volumes", opts.Name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Allocation{}, fmt.Errorf("hoststorage: create allocation dir: %w", err)
		}
		rec.Path = dir
	}

	a, err := m.store.InsertAllocation(rec)
	if err != nil {
		// Roll back the backing storage on record failure.
		if rec.Device != "" {
			_ = lvRemove(context.Background(), rec.Device)
		} else if rec.Path != "" {
			os.RemoveAll(rec.Path)
		}
		return Allocation{}, err
	}
	return a, nil
}

// ListAllocations returns allocations, optionally scoped to a pool.
func (m *Manager) ListAllocations(poolID string) ([]Allocation, error) {
	return m.store.ListAllocations(poolID)
}

// Release removes an allocation and its backing storage, returning capacity to
// the pool.
func (m *Manager) Release(allocationID string) error {
	a, err := m.store.GetAllocation(allocationID)
	if err != nil {
		return fmt.Errorf("hoststorage: allocation not found: %s", allocationID)
	}
	if a.Device != "" {
		if err := lvRemove(context.Background(), a.Device); err != nil {
			return err
		}
	} else if a.Path != "" {
		if err := os.RemoveAll(a.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("hoststorage: remove allocation dir: %w", err)
		}
	}
	return m.store.DeleteAllocation(a.ID)
}

// ReleaseByOwner releases the allocation owned by owner, if any. It is a no-op
// when the owner has no allocation (e.g. instances not backed by a pool).
func (m *Manager) ReleaseByOwner(owner string) error {
	a, err := m.store.GetAllocationByOwner(owner)
	if err != nil {
		return nil // no allocation for this owner
	}
	return m.Release(a.ID)
}

// Reconcile refreshes each pool's capacity from its backend and marks pools
// whose backing storage has gone missing as degraded. It is safe to call
// repeatedly (used by the background reconciler).
func (m *Manager) Reconcile(ctx context.Context) error {
	pools, err := m.store.ListPools()
	if err != nil {
		return err
	}
	for _, p := range pools {
		total, health := m.probe(ctx, p)
		if total != p.TotalBytes || health != p.Health {
			_ = m.store.UpdatePoolHealth(p.ID, total, health)
		}
	}
	return nil
}

// probe inspects a pool's backend and returns its current capacity and health.
func (m *Manager) probe(ctx context.Context, p StoragePool) (int64, string) {
	switch p.Backend {
	case BackendLVM:
		n, err := vgSizeBytes(ctx, p.VGName)
		if err != nil {
			return p.TotalBytes, PoolDegraded
		}
		return n, PoolHealthy
	default:
		info, err := os.Stat(p.Mountpoint)
		if err != nil || !info.IsDir() {
			return p.TotalBytes, PoolDegraded
		}
		if n, cerr := statfsCapacity(p.Mountpoint); cerr == nil {
			return n, PoolHealthy
		}
		return p.TotalBytes, PoolHealthy
	}
}
