package network

// iptfilter.go — iptables FORWARD and INPUT accept rules for capper networks.
//
// Capper uses nftables (nftnat.go) for NAT/masquerade/DNAT, but FORWARD and
// INPUT filtering is owned by UFW (iptables-nft, ip filter table). Rules in a
// separate nftables table can't bypass UFW's DROP policy because nftables ACCEPT
// is non-final across tables at the same hook. However, iptables ACCEPT within
// UFW's own chain IS final — rules inserted at position 1 of FORWARD/INPUT are
// evaluated before UFW's sub-chains and terminate chain traversal on match.
//
// All operations exec iptables-nft and are idempotent (check before insert).
// CAP_NET_ADMIN must be in the ambient set (raised by CheckCapabilities).

import (
	"fmt"
	"os/exec"
	"strings"
)

// AddForwardAccept inserts iptables FORWARD ACCEPT rules for subnet so that
// instances can send packets to the internet and receive replies past UFW.
// Idempotent: checks for existing rules before inserting.
func AddForwardAccept(subnet string) error {
	if err := ipt("FORWARD", "-s", subnet, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("iptfilter: forward out for %s: %w", subnet, err)
	}
	// Return traffic (ESTABLISHED/RELATED) for this subnet.
	if err := ipt("FORWARD", "-d", subnet, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("iptfilter: forward in for %s: %w", subnet, err)
	}
	return nil
}

// RemoveForwardAccept removes the FORWARD ACCEPT rules for subnet.
func RemoveForwardAccept(subnet string) error {
	_ = iptDel("FORWARD", "-s", subnet, "-j", "ACCEPT")
	_ = iptDel("FORWARD", "-d", subnet, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	return nil
}

// AddBridgeInputAccept inserts an INPUT ACCEPT rule allowing all traffic from
// instances on bridge/subnet to reach capper host services (DNS :53, meta :8080).
// Idempotent.
func AddBridgeInputAccept(bridge, subnet string) error {
	if err := ipt("INPUT", "-i", bridge, "-s", subnet, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("iptfilter: input for %s/%s: %w", bridge, subnet, err)
	}
	return nil
}

// RemoveBridgeInputAccept removes the INPUT ACCEPT rule for the bridge/subnet pair.
func RemoveBridgeInputAccept(bridge, subnet string) error {
	_ = iptDel("INPUT", "-i", bridge, "-s", subnet, "-j", "ACCEPT")
	return nil
}

// ipt checks if the iptables rule already exists, and if not inserts it at
// position 1 (before UFW's sub-chain jumps).
func ipt(chain string, args ...string) error {
	// Check existence first to keep the operation idempotent.
	checkArgs := append([]string{"-C", chain}, args...)
	if err := iptables(checkArgs...); err == nil {
		return nil // already present
	}
	insertArgs := append([]string{"-I", chain, "1"}, args...)
	return iptables(insertArgs...)
}

// iptDel removes an iptables rule (best-effort; ignores "not found" errors).
func iptDel(chain string, args ...string) error {
	delArgs := append([]string{"-D", chain}, args...)
	err := iptables(delArgs...)
	if err != nil && (strings.Contains(err.Error(), "No chain/target/match by that name") ||
		strings.Contains(err.Error(), "does a matching rule exist in that chain")) {
		return nil
	}
	return err
}

func iptables(args ...string) error {
	// Use iptables-nft so rules sit in the same table UFW uses.
	// CAP_NET_ADMIN is in the ambient set (raised by network.CheckCapabilities).
	bin := "/usr/sbin/iptables-nft"
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
