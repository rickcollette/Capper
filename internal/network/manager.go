package network

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Manager provides the high-level network lifecycle operations.
type Manager struct {
	store *Store
}

// NewManager creates a Manager.
func NewManager(s *Store) *Manager {
	return &Manager{store: s}
}

// CreateOptions configures network creation.
type CreateOptions struct {
	Subnet string // CIDR; default "10.42.0.0/24"
	Mode   string // "nat" | "isolated" | "host-exposed"; default "nat"
	Labels map[string]string
}

// Create provisions a new network: allocates a bridge, configures the OS,
// and records the network in the store.
func (m *Manager) Create(name, project string, opts CreateOptions) (Network, error) {
	subnet := opts.Subnet
	if subnet == "" {
		subnet = "10.42.0.0/24"
	}
	mode := opts.Mode
	if mode == "" {
		mode = ModeNAT
	}
	if mode != ModeNAT && mode != ModeIsolated && mode != ModeHostExposed {
		return Network{}, fmt.Errorf("invalid network mode %q (valid: nat, isolated, host-exposed)", mode)
	}

	gateway, err := GatewayForSubnet(subnet)
	if err != nil {
		return Network{}, err
	}
	bridge := BridgeName(name)

	n := Network{
		ID:        newNetID(),
		Name:      name,
		Project:   project,
		Mode:      mode,
		Subnet:    subnet,
		Gateway:   gateway,
		Bridge:    bridge,
		Labels:    opts.Labels,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := m.store.Insert(n); err != nil {
		return Network{}, fmt.Errorf("network: store: %w", err)
	}

	if err := CreateBridge(bridge, gateway, subnet, mode); err != nil {
		_ = m.store.UpdateStatusError(n.ID, "error", err.Error())
		n.Status = "error"
		n.Error = err.Error()
		return n, nil
	}

	// Install the DNAT rule that redirects 169.254.169.254:80 → gateway:80 so
	// instances can reach the per-network metadata server via the link-local address.
	if err := AddMetadataDNAT(bridge, gateway); err != nil {
		_ = m.store.UpdateStatusError(n.ID, "error", err.Error())
		n.Status = "error"
		n.Error = err.Error()
		return n, nil
	}

	_ = m.store.UpdateStatus(n.ID, StatusActive)
	n.Status = StatusActive
	return n, nil
}

// Delete removes the OS bridge and the store record.
func (m *Manager) Delete(nameOrID, project string) error {
	n, err := m.store.Get(nameOrID, project)
	if err != nil {
		return err
	}
	_ = RemoveMetadataDNAT(n.Bridge, n.Gateway)
	if err := DeleteBridge(n.Bridge, n.Subnet, n.Mode); err != nil {
		return err
	}
	return m.store.Delete(n.ID)
}

// Connect allocates an IP for instanceID on the named network, creates a veth
// pair, and returns the lease.
func (m *Manager) Connect(instanceID, networkNameOrID, project, preferredIP string) (NetworkLease, error) {
	n, err := m.store.Get(networkNameOrID, project)
	if err != nil {
		return NetworkLease{}, err
	}

	lease, err := AllocateIP(m.store, n, instanceID, preferredIP)
	if err != nil {
		return NetworkLease{}, err
	}

	hostVeth, instanceVeth := VethNames(instanceID)
	if err := CreateVeth(n.Bridge, hostVeth, instanceVeth); err != nil {
		// Roll back the IP allocation on veth failure.
		_ = ReleaseIP(m.store, n.ID, instanceID)
		return NetworkLease{}, err
	}

	return lease, nil
}

// Disconnect releases the IP and removes the veth pair for the given instance.
func (m *Manager) Disconnect(instanceID, networkNameOrID, project string) error {
	n, err := m.store.Get(networkNameOrID, project)
	if err != nil {
		return err
	}

	hostVeth, _ := VethNames(instanceID)
	_ = DeleteVeth(hostVeth) // best-effort; clean up IPAM regardless

	return ReleaseIP(m.store, n.ID, instanceID)
}

// List returns networks in the given project.
func (m *Manager) List(project string) ([]Network, error) {
	return m.store.List(project)
}

// Inspect returns the network and its active leases.
func (m *Manager) Inspect(nameOrID, project string) (Network, []NetworkLease, error) {
	n, err := m.store.Get(nameOrID, project)
	if err != nil {
		return Network{}, nil, err
	}
	leases, err := m.store.LeasesForNetwork(n.ID)
	return n, leases, err
}

func newNetID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "net_" + hex.EncodeToString(b)
}
