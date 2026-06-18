package network

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
)

// AllocateIP finds the next available IP in the network's subnet, records a
// lease, and returns it. If preferredIP is non-empty it is tried first.
// The gateway and broadcast addresses are always skipped.
func AllocateIP(s *Store, n Network, instanceID, preferredIP string) (NetworkLease, error) {
	_, ipnet, err := net.ParseCIDR(n.Subnet)
	if err != nil {
		return NetworkLease{}, fmt.Errorf("ipam: parse subnet %q: %w", n.Subnet, err)
	}

	allocated, err := s.AllocatedIPs(n.ID)
	if err != nil {
		return NetworkLease{}, err
	}

	gateway := n.Gateway
	broadcast := broadcastAddr(ipnet)

	if preferredIP != "" {
		if err := validatePreferred(preferredIP, gateway, broadcast, ipnet, allocated); err != nil {
			return NetworkLease{}, err
		}
		return writeLease(s, n.ID, instanceID, preferredIP)
	}

	// Walk from gateway+1 to broadcast-1.
	candidate := nextIP(net.ParseIP(gateway).To4())
	last := prevIP(broadcast)
	for ; !ipAfter(candidate, last); candidate = nextIP(candidate) {
		addr := candidate.String()
		if !allocated[addr] {
			return writeLease(s, n.ID, instanceID, addr)
		}
	}
	return NetworkLease{}, fmt.Errorf("ipam: no free addresses in %s", n.Subnet)
}

// ReleaseIP removes the lease for instanceID on networkID.
func ReleaseIP(s *Store, networkID, instanceID string) error {
	return s.DeleteLease(networkID, instanceID)
}

// GatewayForSubnet returns the first usable host address of the CIDR
// (e.g. "10.42.0.0/24" → "10.42.0.1").
func GatewayForSubnet(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("ipam: parse CIDR %q: %w", cidr, err)
	}
	return nextIP(ipnet.IP.Mask(ipnet.Mask)).String(), nil
}

// BridgeName derives a valid Linux interface name (≤ 15 chars) from a network name.
func BridgeName(networkName string) string {
	const prefix = "capbr-"
	max := 15 - len(prefix)
	name := networkName
	if len(name) > max {
		name = name[:max]
	}
	return prefix + name
}

// RandomMAC generates a locally-administered unicast MAC address.
func RandomMAC() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[0] = (b[0] | 0x02) & 0xfe // set LAA bit, clear multicast bit
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5]), nil
}

// ---- internal helpers -------------------------------------------------------

func validatePreferred(ip, gateway, broadcast string, ipnet *net.IPNet, allocated map[string]bool) error {
	parsed := net.ParseIP(ip).To4()
	if parsed == nil {
		return fmt.Errorf("ipam: invalid preferred IP %q", ip)
	}
	if !ipnet.Contains(parsed) {
		return fmt.Errorf("ipam: %s is not in subnet %s", ip, ipnet)
	}
	if ip == gateway || ip == broadcast {
		return fmt.Errorf("ipam: %s is a reserved address", ip)
	}
	if allocated[ip] {
		return fmt.Errorf("ipam: %s is already allocated", ip)
	}
	return nil
}

func writeLease(s *Store, networkID, instanceID, ip string) (NetworkLease, error) {
	mac, err := RandomMAC()
	if err != nil {
		return NetworkLease{}, err
	}
	l := NetworkLease{NetworkID: networkID, InstanceID: instanceID, IP: ip, MAC: mac}
	if err := s.InsertLease(l); err != nil {
		return NetworkLease{}, err
	}
	return l, nil
}

func broadcastAddr(ipnet *net.IPNet) string {
	ip := ipnet.IP.To4()
	bc := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		bc[i] = ip[i] | ^ipnet.Mask[i]
	}
	return bc.String()
}

func nextIP(ip net.IP) net.IP {
	ip4 := ip.To4()
	n := binary.BigEndian.Uint32(ip4)
	n++
	out := make(net.IP, 4)
	binary.BigEndian.PutUint32(out, n)
	return out
}

func prevIP(ipStr string) net.IP {
	ip4 := net.ParseIP(ipStr).To4()
	n := binary.BigEndian.Uint32(ip4)
	n--
	out := make(net.IP, 4)
	binary.BigEndian.PutUint32(out, n)
	return out
}

func ipAfter(a, b net.IP) bool {
	na := new(big.Int).SetBytes(a.To4())
	nb := new(big.Int).SetBytes(b.To4())
	return na.Cmp(nb) > 0
}
