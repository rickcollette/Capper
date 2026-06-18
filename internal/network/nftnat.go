package network

// nftnat.go — NAT/DNAT rules via github.com/google/nftables (pure netlink, no subprocess).
// All operations require only CAP_NET_ADMIN on the calling process.

import (
	"bytes"
	"fmt"
	"net"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

const (
	capperNatTableName = "capper_nat"
	postroutingChainName = "postrouting"
	preroutingChainName  = "prerouting"
)

// addMasquerade adds an nftables MASQUERADE rule for traffic sourced from subnet.
// Idempotent: a rule tagged with the same subnet is not added twice.
func addMasquerade(subnet string) error {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return fmt.Errorf("nft: parse subnet %s: %w", subnet, err)
	}

	c, err := nftables.New()
	if err != nil {
		return fmt.Errorf("nft: connect: %w", err)
	}

	table, err := nftEnsureTable(c)
	if err != nil {
		return err
	}
	chain, err := nftEnsureChain(c, table, postroutingChainName,
		nftables.ChainHookPostrouting, nftables.ChainPriorityNATSource)
	if err != nil {
		return err
	}

	tag := []byte("capper-masq:" + subnet)
	if nftRuleExists(c, table, chain, tag) {
		return nil
	}

	c.AddRule(&nftables.Rule{
		Table:    table,
		Chain:    chain,
		UserData: tag,
		Exprs: []expr.Any{
			// ip saddr <subnet>
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4},
			&expr.Bitwise{
				SourceRegister: 1, DestRegister: 1, Len: 4,
				Mask: []byte(ipNet.Mask),
				Xor:  []byte{0, 0, 0, 0},
			},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ipNet.IP.To4()},
			// masquerade
			&expr.Masq{},
		},
	})
	return c.Flush()
}

// removeMasquerade removes the MASQUERADE rule for the given subnet.
func removeMasquerade(subnet string) error {
	return nftDeleteTagged(postroutingChainName, []byte("capper-masq:"+subnet))
}

// AddMetadataDNAT adds an nftables DNAT rule redirecting packets arriving on
// bridge destined for 169.254.169.254:80 to gatewayIP:80.
// Idempotent: existing rules for the same bridge are not duplicated.
func AddMetadataDNAT(bridge, gatewayIP string) error {
	gw := net.ParseIP(gatewayIP).To4()
	if gw == nil {
		return fmt.Errorf("nft: invalid gateway IP %s", gatewayIP)
	}

	c, err := nftables.New()
	if err != nil {
		return fmt.Errorf("nft: connect: %w", err)
	}

	table, err := nftEnsureTable(c)
	if err != nil {
		return err
	}
	chain, err := nftEnsureChain(c, table, preroutingChainName,
		nftables.ChainHookPrerouting, nftables.ChainPriorityNATDest)
	if err != nil {
		return err
	}

	tag := []byte("capper-dnat:" + bridge + ":8080")
	if nftRuleExists(c, table, chain, tag) {
		return nil
	}

	// Interface name padded to IFNAMSIZ (16 bytes, null-terminated).
	ifname := make([]byte, 16)
	copy(ifname, bridge)

	c.AddRule(&nftables.Rule{
		Table:    table,
		Chain:    chain,
		UserData: tag,
		Exprs: []expr.Any{
			// iifname == bridge
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifname},
			// ip daddr == 169.254.169.254
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: net.ParseIP("169.254.169.254").To4()},
			// tcp dport == 80
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_TCP}},
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.BigEndian.PutUint16(80)},
			// dnat to gateway:8080 (metadata server listens on unprivileged 8080)
			&expr.Immediate{Register: 1, Data: gw},
			&expr.Immediate{Register: 2, Data: binaryutil.BigEndian.PutUint16(8080)},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  1,
				RegProtoMin: 2,
			},
		},
	})
	return c.Flush()
}

// RestoreNetworkRules re-applies all nftables/iptables rules for n.
// Called on each reconcile tick to restore rules lost after a reboot.
func RestoreNetworkRules(n Network) {
	if n.Bridge != "" && n.Gateway != "" {
		_ = AddMetadataDNAT(n.Bridge, n.Gateway)
	}
	if n.Mode == ModeNAT || n.Mode == ModeHostExposed {
		_ = EnableIPForwarding()
		_ = addMasquerade(n.Subnet)
		_ = AddForwardAccept(n.Subnet)
		_ = AddBridgeInputAccept(n.Bridge, n.Subnet)
	}
}

// RemoveMetadataDNAT removes the DNAT rule for the given bridge.
func RemoveMetadataDNAT(bridge, _ string) error {
	// Delete both the current tag and the old port-80 tag (used before v3.24).
	_ = nftDeleteTagged(preroutingChainName, []byte("capper-dnat:"+bridge))
	return nftDeleteTagged(preroutingChainName, []byte("capper-dnat:"+bridge+":8080"))
}

// ---- helpers ----------------------------------------------------------------

func nftEnsureTable(c *nftables.Conn) (*nftables.Table, error) {
	tables, err := c.ListTables()
	if err != nil {
		return nil, fmt.Errorf("nft: list tables: %w", err)
	}
	for _, t := range tables {
		if t.Name == capperNatTableName && t.Family == nftables.TableFamilyIPv4 {
			return t, nil
		}
	}
	t := c.AddTable(&nftables.Table{Name: capperNatTableName, Family: nftables.TableFamilyIPv4})
	if err := c.Flush(); err != nil {
		return nil, fmt.Errorf("nft: create table: %w", err)
	}
	return t, nil
}

func nftEnsureChain(c *nftables.Conn, table *nftables.Table, name string, hook *nftables.ChainHook, prio *nftables.ChainPriority) (*nftables.Chain, error) {
	chains, err := c.ListChains()
	if err != nil {
		return nil, fmt.Errorf("nft: list chains: %w", err)
	}
	for _, ch := range chains {
		if ch.Table.Name == table.Name && ch.Name == name {
			return ch, nil
		}
	}
	policy := nftables.ChainPolicyAccept
	ch := c.AddChain(&nftables.Chain{
		Name:     name,
		Table:    table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  hook,
		Priority: prio,
		Policy:   &policy,
	})
	if err := c.Flush(); err != nil {
		return nil, fmt.Errorf("nft: create chain %s: %w", name, err)
	}
	return ch, nil
}

func nftRuleExists(c *nftables.Conn, table *nftables.Table, chain *nftables.Chain, tag []byte) bool {
	rules, err := c.GetRules(table, chain)
	if err != nil {
		return false
	}
	for _, r := range rules {
		if bytes.Equal(r.UserData, tag) {
			return true
		}
	}
	return false
}

func nftDeleteTagged(chainName string, tag []byte) error {
	c, err := nftables.New()
	if err != nil {
		return nil
	}
	tables, _ := c.ListTables()
	var table *nftables.Table
	for _, t := range tables {
		if t.Name == capperNatTableName {
			table = t
			break
		}
	}
	if table == nil {
		return nil
	}
	chains, _ := c.ListChains()
	var chain *nftables.Chain
	for _, ch := range chains {
		if ch.Table.Name == table.Name && ch.Name == chainName {
			chain = ch
			break
		}
	}
	if chain == nil {
		return nil
	}
	rules, err := c.GetRules(table, chain)
	if err != nil {
		return nil
	}
	for _, r := range rules {
		if bytes.Equal(r.UserData, tag) {
			if err := c.DelRule(r); err == nil {
				_ = c.Flush()
			}
			return nil
		}
	}
	return nil
}
