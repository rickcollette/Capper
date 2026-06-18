package network

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const netnsRunDir = "/run/capper/netns"

// NetNSPath returns the bind-mount path for a named capper network namespace.
func NetNSPath(name string) string {
	return filepath.Join(netnsRunDir, name)
}

// NetNSName derives the named network namespace identifier for an instance.
func NetNSName(instanceID string) string {
	short := instanceID
	if idx := strings.Index(instanceID, "_"); idx >= 0 {
		short = instanceID[idx+1:]
	}
	if len(short) > 12 {
		short = short[:12]
	}
	return "capns-" + short
}

// SetupInstanceNetNS creates a named network namespace for the instance, moves
// instanceVeth into it, assigns ip/prefix, brings the interface up, and adds a
// default route via gateway.
func SetupInstanceNetNS(instanceID, instanceVeth, ip, prefix, gateway string) error {
	// All netns operations must stay on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	name := NetNSName(instanceID)

	if err := os.MkdirAll(netnsRunDir, 0o755); err != nil {
		return fmt.Errorf("netns: mkdir %s: %w", netnsRunDir, err)
	}

	nsPath := filepath.Join(netnsRunDir, name)

	// Create the bind-mount target file if it doesn't exist.
	if _, err := os.Stat(nsPath); os.IsNotExist(err) {
		f, err := os.OpenFile(nsPath, os.O_CREATE|os.O_RDONLY, 0o444)
		if err != nil {
			return fmt.Errorf("netns: create mountpoint %s: %w", nsPath, err)
		}
		f.Close()

		// Save the original namespace so we can restore after netns.New() switches into the new one.
		origNS, err := netns.Get()
		if err != nil {
			_ = os.Remove(nsPath)
			return fmt.Errorf("netns: get current ns: %w", err)
		}
		defer origNS.Close()

		// netns.New() creates a new ns AND switches the current OS thread into it.
		newNS, err := netns.New()
		if err != nil {
			_ = os.Remove(nsPath)
			return fmt.Errorf("netns: create new netns: %w", err)
		}

		// Bind-mount the new ns to the named path before switching back.
		nsFd := fmt.Sprintf("/proc/self/fd/%d", int(newNS))
		mountErr := unix.Mount(nsFd, nsPath, "", unix.MS_BIND, "")

		// Restore original ns immediately — all subsequent operations must run in the host ns.
		_ = netns.Set(origNS)
		newNS.Close()

		if mountErr != nil {
			_ = os.Remove(nsPath)
			return fmt.Errorf("netns: bind mount netns: %w", mountErr)
		}
	}

	ns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("netns: open %s: %w", name, err)
	}
	defer ns.Close()

	// Move instanceVeth into the namespace from the host ns (requires only CAP_NET_ADMIN).
	peerLink, err := netlink.LinkByName(instanceVeth)
	if err != nil {
		return fmt.Errorf("netns: lookup %s: %w", instanceVeth, err)
	}
	if err := netlink.LinkSetNsFd(peerLink, int(ns)); err != nil {
		return fmt.Errorf("netns: move %s into %s: %w", instanceVeth, name, err)
	}

	// Configure the namespace: lo up, instanceVeth up + addr + routes.
	nh, err := netlink.NewHandleAt(ns)
	if err != nil {
		return fmt.Errorf("netns: handle in %s: %w", name, err)
	}
	defer nh.Close()

	lo, err := nh.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("netns: lookup lo: %w", err)
	}
	if err := nh.LinkSetUp(lo); err != nil {
		return fmt.Errorf("netns: lo up: %w", err)
	}

	peer, err := nh.LinkByName(instanceVeth)
	if err != nil {
		return fmt.Errorf("netns: lookup %s in ns: %w", instanceVeth, err)
	}
	if err := nh.LinkSetUp(peer); err != nil {
		return fmt.Errorf("netns: bring up %s: %w", instanceVeth, err)
	}

	ifAddr, err := netlink.ParseAddr(ip + "/" + prefix)
	if err != nil {
		return fmt.Errorf("netns: parse addr %s/%s: %w", ip, prefix, err)
	}
	if err := nh.AddrAdd(peer, ifAddr); err != nil && !isExist(err) {
		return fmt.Errorf("netns: assign %s/%s to %s: %w", ip, prefix, instanceVeth, err)
	}

	gw := net.ParseIP(gateway)
	if err := nh.RouteAdd(&netlink.Route{
		LinkIndex: peer.Attrs().Index,
		Gw:        gw,
	}); err != nil && !isExist(err) {
		return fmt.Errorf("netns: default route via %s: %w", gateway, err)
	}

	// Route 169.254.169.254/32 via gateway for metadata service.
	_, meta, _ := net.ParseCIDR("169.254.169.254/32")
	_ = nh.RouteAdd(&netlink.Route{
		LinkIndex: peer.Attrs().Index,
		Dst:       meta,
		Gw:        gw,
	})

	// Allow unprivileged ICMP ping via ICMP_DGRAM sockets.
	// ping_group_range is network-namespace scoped, so this only affects
	// the instance's namespace, not the host.
	curNS, curErr := netns.Get()
	if curErr == nil {
		if err := netns.Set(ns); err == nil {
			_ = os.WriteFile("/proc/sys/net/ipv4/ping_group_range", []byte("0\t65535"), 0o644)
			_ = netns.Set(curNS)
		}
		curNS.Close()
	}

	return nil
}

// TeardownInstanceNetNS deletes the named network namespace for the instance.
func TeardownInstanceNetNS(instanceID string) error {
	name := NetNSName(instanceID)
	nsPath := filepath.Join(netnsRunDir, name)

	if _, err := os.Stat(nsPath); os.IsNotExist(err) {
		return nil // already gone
	}

	if err := unix.Unmount(nsPath, unix.MNT_DETACH); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("netns: unmount %s: %w", name, err)
	}
	if err := os.Remove(nsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("netns: remove %s: %w", nsPath, err)
	}
	return nil
}
