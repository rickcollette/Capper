package ipam

import (
	"database/sql"
	"fmt"
)

// Manager orchestrates pool creation, reservation, and binding while enforcing
// allocation rules.
type Manager struct {
	store *Store
}

// NewManager wraps a Store.
func NewManager(store *Store) *Manager { return &Manager{store: store} }

// Store exposes the underlying store.
func (m *Manager) Store() *Store { return m.store }

// CreatePoolOptions configures pool creation and address materialization.
type CreatePoolOptions struct {
	Pool     RoutableIPPool
	Excluded []string
	MaxHosts int
}

// CreatePool stores a pool and materializes its usable addresses (excluding
// network/broadcast/gateway and any excluded addresses).
func (m *Manager) CreatePool(opts CreatePoolOptions) (RoutableIPPool, int, error) {
	addrs, err := ExpandCIDR(opts.Pool.CIDR, opts.Pool.Gateway, opts.Excluded, opts.MaxHosts)
	if err != nil {
		return RoutableIPPool{}, 0, err
	}
	pool, err := m.store.InsertPool(opts.Pool)
	if err != nil {
		return RoutableIPPool{}, 0, err
	}
	for _, a := range addrs {
		if err := m.store.InsertIP(RoutableIP{PoolID: pool.ID, Address: a, Status: IPAvailable}); err != nil {
			return pool, 0, err
		}
	}
	return pool, len(addrs), nil
}

// usageAllows reports whether the pool permits the requested purpose.
func usageAllows(pool RoutableIPPool, purpose string) bool {
	if purpose == "" {
		return true
	}
	for _, u := range pool.Usage {
		if u == purpose {
			return true
		}
	}
	return false
}

// ReserveOptions configures an IP reservation.
type ReserveOptions struct {
	PoolID    string
	Project   string
	Name      string
	Purpose   string
	Address   string // optional: reserve a specific address
	Reserved  bool   // true => allocation_type=reserved (no auto reuse)
}

// Reserve allocates an address from a pool, enforcing allocation rules:
//   - the pool must be active (not draining/disabled/retired);
//   - the pool's usage must permit the purpose;
//   - reserved-only pools (allowAutoAllocate=false) require an explicit address;
//   - the requested address must be available.
func (m *Manager) Reserve(opts ReserveOptions) (RoutableIP, error) {
	pool, err := m.store.GetPool(opts.PoolID)
	if err != nil {
		return RoutableIP{}, fmt.Errorf("ipam: pool not found: %s", opts.PoolID)
	}
	if pool.Status != PoolActive {
		return RoutableIP{}, fmt.Errorf("ipam: pool %q is %s; cannot allocate", pool.Name, pool.Status)
	}
	if !usageAllows(pool, opts.Purpose) {
		return RoutableIP{}, fmt.Errorf("ipam: pool %q does not permit purpose %q", pool.Name, opts.Purpose)
	}

	var ip RoutableIP
	if opts.Address != "" {
		ip, err = m.store.GetAvailableAddress(pool.ID, opts.Address)
		if err != nil {
			return RoutableIP{}, fmt.Errorf("ipam: address %s is not available in pool %q", opts.Address, pool.Name)
		}
	} else {
		if !pool.AllowAutoAllocate {
			return RoutableIP{}, fmt.Errorf("ipam: pool %q is reserved-only; specify an explicit address", pool.Name)
		}
		ip, err = m.store.FirstAvailable(pool.ID)
		if err == sql.ErrNoRows {
			_ = m.store.SetPoolStatus(pool.ID, PoolExhausted)
			return RoutableIP{}, fmt.Errorf("ipam: pool %q is exhausted", pool.Name)
		}
		if err != nil {
			return RoutableIP{}, err
		}
	}

	ip.Status = IPReserved
	ip.Project = opts.Project
	ip.Name = opts.Name
	ip.Purpose = opts.Purpose
	ip.AllocationType = "auto"
	if opts.Reserved {
		ip.AllocationType = "reserved"
	}
	if err := m.store.UpdateIP(ip); err != nil {
		return RoutableIP{}, err
	}
	return m.store.GetIP(ip.ID)
}

// Release returns an address to the available pool, removing any bindings.
func (m *Manager) Release(ipID string) error {
	ip, err := m.store.GetIP(ipID)
	if err != nil {
		return err
	}
	if err := m.store.DeleteBindingsForIP(ipID); err != nil {
		return err
	}
	ip.Status = IPAvailable
	ip.Project, ip.Name, ip.Purpose = "", "", ""
	ip.TargetType, ip.TargetID = "", ""
	ip.AllocationType = "auto"
	if err := m.store.UpdateIP(ip); err != nil {
		return err
	}
	// Reactivate an exhausted pool now that an address is free.
	if pool, perr := m.store.GetPool(ip.PoolID); perr == nil && pool.Status == PoolExhausted {
		_ = m.store.SetPoolStatus(pool.ID, PoolActive)
	}
	return nil
}

// Attach binds a reserved address to a target. It enforces the conflict rule
// that the same IP+protocol+external-port cannot bind to two targets.
func (m *Manager) Attach(ipID string, b IPBinding) (IPBinding, error) {
	ip, err := m.store.GetIP(ipID)
	if err != nil {
		return IPBinding{}, err
	}
	if ip.Status == IPAvailable || ip.Status == IPRetired || ip.Status == IPBlocked {
		return IPBinding{}, fmt.Errorf("ipam: address %s is %s and cannot be attached", ip.Address, ip.Status)
	}
	existing, err := m.store.ListBindings(ipID)
	if err != nil {
		return IPBinding{}, err
	}
	for _, e := range existing {
		if e.Protocol == b.Protocol && e.ExternalPort == b.ExternalPort &&
			(e.TargetType != b.TargetType || e.TargetID != b.TargetID) {
			return IPBinding{}, fmt.Errorf("ipam: %s:%d/%s already bound to a different target",
				ip.Address, b.ExternalPort, b.Protocol)
		}
	}
	b.IPID = ipID
	binding, err := m.store.InsertBinding(b)
	if err != nil {
		return IPBinding{}, err
	}
	ip.Status = IPAttached
	ip.TargetType = b.TargetType
	ip.TargetID = b.TargetID
	if err := m.store.UpdateIP(ip); err != nil {
		return IPBinding{}, err
	}
	return binding, nil
}

// Detach removes all bindings from an address and returns it to reserved state.
func (m *Manager) Detach(ipID string) error {
	ip, err := m.store.GetIP(ipID)
	if err != nil {
		return err
	}
	if err := m.store.DeleteBindingsForIP(ipID); err != nil {
		return err
	}
	ip.Status = IPReserved
	ip.TargetType, ip.TargetID = "", ""
	return m.store.UpdateIP(ip)
}
