// Package network manages Capper virtual networks: IP address management,
// Linux bridge creation, NAT configuration, and veth instance wiring.
package network

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

const (
	capNetAdmin = uint32(1 << 12)
	capNetRaw   = uint32(1 << 13)
)

// CheckCapabilities verifies that the process has CAP_NET_ADMIN, which is
// required for bridge/veth/netns operations. Call at startup; returns a
// descriptive error with the fix command if the capability is absent.
// On success it also raises the capabilities as ambient so that subprocesses
// (iptables, ip, etc.) inherit them without needing their own file capabilities.
func CheckCapabilities() error {
	if os.Getuid() == 0 {
		return nil // root has everything
	}
	// LINUX_CAPABILITY_VERSION_3 uses a 2-element array of CapUserData.
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return fmt.Errorf("network: capability check failed: %w", err)
	}
	if data[0].Effective&capNetAdmin == 0 {
		return fmt.Errorf(
			"network: CAP_NET_ADMIN not set on this binary — run:\n"+
				"  sudo setcap cap_net_admin,cap_net_raw+eip %s\n"+
				"or use: make setcap",
			exePath(),
		)
	}
	// Add to inheritable set — allowed because the caps are already in our permitted set.
	// Then raise in ambient so all child processes (iptables, ip) inherit them.
	data[0].Inheritable |= capNetAdmin | capNetRaw
	if err := unix.Capset(&hdr, &data[0]); err == nil {
		_ = unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_RAISE, uintptr(unix.CAP_NET_ADMIN), 0, 0)
		_ = unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_RAISE, uintptr(unix.CAP_NET_RAW), 0, 0)
	}
	return nil
}

func exePath() string {
	p, err := os.Executable()
	if err != nil {
		return "<binary>"
	}
	return p
}

// Network is a managed virtual network backed by a Linux bridge.
type Network struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Project   string            `json:"project"`
	Mode      string            `json:"mode"`    // "nat" | "isolated" | "host-exposed"
	Subnet    string            `json:"subnet"`  // CIDR, e.g. "10.42.0.0/24"
	Gateway   string            `json:"gateway"` // first usable IP
	Bridge    string            `json:"bridge"`  // Linux bridge interface name
	Labels    map[string]string `json:"labels,omitempty"`
	Status    string            `json:"status"`
	Error     string            `json:"error,omitempty"` // last error message if status is "error"
	CreatedAt string            `json:"createdAt"`
}

// NetworkLease records the assignment of an IP to an instance on a network.
type NetworkLease struct {
	NetworkID  string `json:"networkId"`
	InstanceID string `json:"instanceId"`
	IP         string `json:"ip"`
	MAC        string `json:"mac"`
	CreatedAt  string `json:"createdAt"`
}

// Network modes.
const (
	ModeNAT         = "nat"
	ModeIsolated    = "isolated"
	ModeHostExposed = "host-exposed"
)

// Network status values.
const (
	StatusActive  = "active"
	StatusPending = "pending"
	StatusDeleted = "deleted"
)
