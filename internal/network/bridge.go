package network

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// CreateBridge creates a Linux bridge interface, assigns the gateway IP, brings
// it up, and for NAT mode enables IP forwarding and adds a MASQUERADE rule.
// Requires CAP_NET_ADMIN on the process binary (set via setcap in the build).
func CreateBridge(bridgeName, gateway, subnet, mode string) error {
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	br := &netlink.Bridge{LinkAttrs: la}

	if err := netlink.LinkAdd(br); err != nil {
		if !isExist(err) {
			return fmt.Errorf("network: create bridge %s: %w", bridgeName, err)
		}
	}

	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("network: lookup bridge %s: %w", bridgeName, err)
	}

	prefix := SubnetPrefix(subnet)
	addr, err := netlink.ParseAddr(gateway + "/" + prefix)
	if err != nil {
		return fmt.Errorf("network: parse gateway addr: %w", err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil && !isExist(err) {
		return fmt.Errorf("network: assign gateway %s to %s: %w", gateway, bridgeName, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("network: bring up bridge %s: %w", bridgeName, err)
	}

	if mode == ModeNAT || mode == ModeHostExposed {
		if err := EnableIPForwarding(); err != nil {
			return err
		}
		if err := addMasquerade(subnet); err != nil {
			return err
		}
		if err := AddForwardAccept(subnet); err != nil {
			return err
		}
		if err := AddBridgeInputAccept(bridgeName, subnet); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBridge removes the bridge interface and, for NAT mode, removes the
// MASQUERADE rule.
func DeleteBridge(bridgeName, subnet, mode string) error {
	if mode == ModeNAT || mode == ModeHostExposed {
		_ = removeMasquerade(subnet)
		_ = RemoveForwardAccept(subnet)
		_ = RemoveBridgeInputAccept(bridgeName, subnet)
	}
	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return fmt.Errorf("network: lookup bridge %s: %w", bridgeName, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("network: delete bridge %s: %w", bridgeName, err)
	}
	return nil
}

// CreateVeth creates a veth pair and attaches the host side to the bridge.
func CreateVeth(bridgeName, hostVeth, instanceVeth string) error {
	la := netlink.NewLinkAttrs()
	la.Name = hostVeth
	veth := &netlink.Veth{LinkAttrs: la, PeerName: instanceVeth}

	if err := netlink.LinkAdd(veth); err != nil && !isExist(err) {
		return fmt.Errorf("network: create veth pair %s/%s: %w", hostVeth, instanceVeth, err)
	}

	hostLink, err := netlink.LinkByName(hostVeth)
	if err != nil {
		return fmt.Errorf("network: lookup %s: %w", hostVeth, err)
	}
	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("network: lookup bridge %s: %w", bridgeName, err)
	}
	if err := netlink.LinkSetMaster(hostLink, bridge); err != nil {
		return fmt.Errorf("network: attach %s to bridge %s: %w", hostVeth, bridgeName, err)
	}
	if err := netlink.LinkSetUp(hostLink); err != nil {
		return fmt.Errorf("network: bring up %s: %w", hostVeth, err)
	}
	return nil
}

// DeleteVeth removes the host-side veth (the peer disappears automatically).
func DeleteVeth(hostVeth string) error {
	link, err := netlink.LinkByName(hostVeth)
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return fmt.Errorf("network: lookup veth %s: %w", hostVeth, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("network: delete veth %s: %w", hostVeth, err)
	}
	return nil
}

// VethNames derives Linux interface names for a veth pair from the instance ID.
func VethNames(instanceID string) (hostVeth, instanceVeth string) {
	short := instanceID
	if idx := strings.Index(instanceID, "_"); idx >= 0 {
		short = instanceID[idx+1:]
	}
	if len(short) > 8 {
		short = short[:8]
	}
	return "cvh-" + short, "cvi-" + short
}

// HotAttachNetNS moves instanceVeth into the network namespace of a running
// process, then configures the interface inside that namespace.
func HotAttachNetNS(pid int, instanceVeth, ip, prefix, gateway string) error {
	peerLink, err := netlink.LinkByName(instanceVeth)
	if err != nil {
		return fmt.Errorf("hot-attach: lookup %s: %w", instanceVeth, err)
	}

	procNS, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("hot-attach: open netns of pid %d: %w", pid, err)
	}
	defer procNS.Close()

	if err := netlink.LinkSetNsFd(peerLink, int(procNS)); err != nil {
		return fmt.Errorf("hot-attach: move %s to pid %d netns: %w", instanceVeth, pid, err)
	}

	// Configure the interface from within the target namespace.
	nh, err := netlink.NewHandleAt(procNS)
	if err != nil {
		return fmt.Errorf("hot-attach: netlink handle in netns: %w", err)
	}
	defer nh.Close()

	peer, err := nh.LinkByName(instanceVeth)
	if err != nil {
		return fmt.Errorf("hot-attach: lookup %s in netns: %w", instanceVeth, err)
	}
	if err := nh.LinkSetUp(peer); err != nil {
		return fmt.Errorf("hot-attach: bring up %s: %w", instanceVeth, err)
	}
	addr, err := netlink.ParseAddr(ip + "/" + prefix)
	if err != nil {
		return fmt.Errorf("hot-attach: parse addr: %w", err)
	}
	if err := nh.AddrAdd(peer, addr); err != nil {
		return fmt.Errorf("hot-attach: assign addr: %w", err)
	}
	gw := net.ParseIP(gateway)
	if err := nh.RouteAdd(&netlink.Route{
		LinkIndex: peer.Attrs().Index,
		Gw:        gw,
	}); err != nil && !isExist(err) {
		return fmt.Errorf("hot-attach: default route: %w", err)
	}
	return nil
}

// HotDetachNetNS removes the instance-side veth by deleting the host-side veth.
func HotDetachNetNS(hostVeth string) error {
	return DeleteVeth(hostVeth)
}

// RemoveStaleBridges removes any Linux bridge interfaces whose names start with
// "capbr-" but are not in the activeBridges set. Stale bridges with the same
// subnet as a live bridge corrupt the host routing table and break NAT.
func RemoveStaleBridges(activeBridges map[string]bool) error {
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("network: list links: %w", err)
	}
	for _, link := range links {
		name := link.Attrs().Name
		if link.Type() != "bridge" || len(name) < 6 || name[:6] != "capbr-" {
			continue
		}
		if activeBridges[name] {
			continue
		}
		// Remove attached veth ports first (the peer inside instance netns
		// disappears automatically when the host side is deleted).
		if err := netlink.LinkDel(link); err != nil && !isNotExist(err) {
			fmt.Fprintf(os.Stderr, "network: remove stale bridge %s: %v\n", name, err)
		}
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

func enableIPForwarding() error {
	return EnableIPForwarding()
}

// EnableIPForwarding enables IPv4 packet forwarding globally.
func EnableIPForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644)
}


func isExist(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "file exists") ||
		strings.Contains(s, "already exists") ||
		strings.Contains(s, "RTNETLINK answers: File exists")
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "no such") ||
		strings.Contains(s, "not found") ||
		strings.Contains(s, "does not exist")
}

// SubnetPrefix returns the prefix length string from a CIDR (e.g. "24" from "10.0.0.0/24").
func SubnetPrefix(cidr string) string {
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return "24"
}
