package ipam

import (
	"fmt"
	"net/netip"
)

// ExpandCIDR returns the usable host addresses in a CIDR, excluding the network
// and broadcast addresses for IPv4 prefixes shorter than /31, the gateway, and
// any explicitly excluded addresses. The result is capped to avoid materializing
// enormous ranges.
func ExpandCIDR(cidr, gateway string, excluded []string, max int) ([]string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("ipam: invalid CIDR %q: %w", cidr, err)
	}
	prefix = prefix.Masked()
	if max <= 0 {
		max = 1024
	}

	excl := map[string]bool{}
	for _, e := range excluded {
		if a, err := netip.ParseAddr(e); err == nil {
			excl[a.String()] = true
		}
	}
	if gw, err := netip.ParseAddr(gateway); err == nil {
		excl[gw.String()] = true
	}

	is4 := prefix.Addr().Is4()
	bits := prefix.Bits()
	// For IPv4 with room for network+broadcast (prefix < /31), exclude them.
	excludeNetBroadcast := is4 && bits < 31

	var network, broadcast netip.Addr
	if excludeNetBroadcast {
		network = prefix.Addr()
		broadcast = lastAddr(prefix)
	}

	var out []string
	for addr := prefix.Addr(); prefix.Contains(addr); addr = addr.Next() {
		if len(out) >= max {
			break
		}
		if excludeNetBroadcast && (addr == network || addr == broadcast) {
			continue
		}
		if excl[addr.String()] {
			continue
		}
		out = append(out, addr.String())
	}
	return out, nil
}

// lastAddr returns the broadcast (last) address of an IPv4 prefix.
func lastAddr(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	if !addr.Is4() {
		return addr
	}
	b := addr.As4()
	hostBits := 32 - prefix.Bits()
	for i := 0; i < hostBits; i++ {
		byteIdx := 3 - i/8
		bitIdx := uint(i % 8)
		b[byteIdx] |= 1 << bitIdx
	}
	return netip.AddrFrom4(b)
}

// CIDRContains reports whether an address falls inside a CIDR.
func CIDRContains(cidr, address string) bool {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return false
	}
	return prefix.Contains(addr)
}
