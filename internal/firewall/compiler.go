package firewall

import (
	"fmt"
	"strings"
	"time"
)

// CompileInput bundles everything the compiler needs. Keeping this a plain
// struct avoids any import of the network or resource packages.
type CompileInput struct {
	Firewall Firewall
	Net      NetworkInfo
	Rules    []Rule             // must be sorted by priority (ascending)
	LeaseIPs map[string]string  // instanceID → IP
	LabelIPs map[string][]string // "key=value" → []IP
}

// Compile generates a complete nftables script that, when piped to `nft -f -`,
// creates and populates the per-network Capper chain. Calling Apply with the
// returned script is idempotent: it flushes and rebuilds the chain each time.
func Compile(in CompileInput) string {
	chain := ChainName(in.Net.ID)
	bridge := in.Net.Bridge

	var b strings.Builder

	b.WriteString(fmt.Sprintf(
		"# Capper firewall — network %s (%s)\n# Generated: %s\n# Mode: %s\n\n",
		in.Net.Name, in.Net.ID, time.Now().UTC().Format(time.RFC3339), in.Firewall.Mode))

	// Ensure the capper table exists.
	b.WriteString("add table inet capper\n\n")

	// Create the per-network chain hooked into the kernel's forward path.
	// `add chain` is idempotent if the chain already exists.
	b.WriteString(fmt.Sprintf(
		"add chain inet capper %s { type filter hook forward priority filter; policy accept; }\n",
		chain))

	// Flush all existing rules so we can rebuild cleanly.
	b.WriteString(fmt.Sprintf("flush chain inet capper %s\n\n", chain))

	// Allow established/related traffic (reduces state-match overhead).
	if in.Firewall.AllowEstablished {
		b.WriteString(fmt.Sprintf(
			"add rule inet capper %s iifname %q ct state { established, related } accept  # stateful\n",
			chain, bridge))
	}

	// Allow DNS to the gateway if requested.
	if in.Firewall.AllowDNS {
		b.WriteString(fmt.Sprintf(
			"add rule inet capper %s iifname %q ip daddr %s udp dport 53 accept  # DNS\n",
			chain, bridge, in.Net.Gateway))
		b.WriteString(fmt.Sprintf(
			"add rule inet capper %s iifname %q ip daddr %s tcp dport 53 accept  # DNS/TCP\n",
			chain, bridge, in.Net.Gateway))
	}

	if in.Firewall.AllowEstablished || in.Firewall.AllowDNS {
		b.WriteString("\n")
	}

	// User rules (sorted by priority).
	for _, r := range in.Rules {
		if !r.Enabled {
			b.WriteString(fmt.Sprintf("# [disabled] rule %s: %s\n", r.ID, r.Description))
			continue
		}
		lines := compileRule(r, chain, bridge, in.Net, in.LeaseIPs, in.LabelIPs)
		if len(lines) == 0 {
			b.WriteString(fmt.Sprintf("# [unresolved] rule %s: %s\n", r.ID, r.Description))
			continue
		}
		if r.Description != "" {
			b.WriteString(fmt.Sprintf("# %s\n", r.Description))
		}
		for _, line := range lines {
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")

	// Default policy: drop all remaining traffic touching this bridge.
	// This covers forward (inter-instance) and egress (instance → internet/host).
	fwdDefault := in.Firewall.DefaultForwardPolicy
	if fwdDefault == "" {
		fwdDefault = ActionDeny
	}
	if fwdDefault == ActionDeny {
		b.WriteString(fmt.Sprintf(
			"add rule inet capper %s iifname %q drop  # default deny forward\n", chain, bridge))
		b.WriteString(fmt.Sprintf(
			"add rule inet capper %s oifname %q drop  # default deny egress\n", chain, bridge))
	}

	return b.String()
}

// ChainName derives a valid nftables chain name from a network ID.
// Format: capfw_<first8hex> e.g. "capfw_aabbccdd" (14 chars).
func ChainName(networkID string) string {
	id := networkID
	if idx := strings.Index(id, "_"); idx >= 0 {
		id = id[idx+1:]
	}
	if len(id) > 8 {
		id = id[:8]
	}
	return "capfw_" + id
}

// compileRule converts a single Rule into one or more nft add-rule strings.
// Returns nil if the rule's endpoints cannot be resolved to any IPs.
func compileRule(r Rule, chain, bridge string, net NetworkInfo, leaseIPs map[string]string, labelIPs map[string][]string) []string {
	fromExprs := resolveEndpoint(r.From, net, leaseIPs, labelIPs, "src")
	toExprs := resolveEndpoint(r.To, net, leaseIPs, labelIPs, "dst")

	// If endpoint could not be resolved to any IPs, skip the rule.
	if fromExprs == nil || toExprs == nil {
		return nil
	}

	nftAction := ruleAction(r.Action)
	protoPort := protoPortExpr(r.Protocol, r.Ports)

	var lines []string

	// Determine interface direction(s) for this rule.
	dirs := ruleDirs(r.Direction, r.To)

	for _, iface := range dirs {
		// For egress to "internet": match traffic leaving the bridge subnet.
		var ifaceMatch string
		if r.To.Type == EndpointInternet {
			ifaceMatch = fmt.Sprintf("iifname %q oifname != %q", bridge, bridge)
		} else {
			ifaceMatch = fmt.Sprintf("%s %q", iface, bridge)
		}

		// Build the rule parts.
		parts := []string{"add rule inet capper", chain, ifaceMatch}
		for _, fe := range fromExprs {
			if fe != "" {
				parts = append(parts, fe)
			}
		}
		for _, te := range toExprs {
			if te != "" {
				parts = append(parts, te)
			}
		}
		if protoPort != "" {
			parts = append(parts, protoPort)
		}
		parts = append(parts, nftAction)
		lines = append(lines, strings.Join(parts, " "))
	}
	return lines
}

// resolveEndpoint converts an Endpoint to a slice of nft match expressions
// (e.g. ["ip saddr 10.42.0.2"] or ["ip saddr { 10.42.0.2, 10.42.0.3 }"])
// Returns nil when the endpoint type requires dynamic IPs that weren't provided.
// Returns []string{""} for endpoint types that match everything (e.g. "any").
func resolveEndpoint(ep Endpoint, net NetworkInfo, leaseIPs map[string]string, labelIPs map[string][]string, direction string) []string {
	var field string
	if direction == "src" {
		field = "ip saddr"
	} else {
		field = "ip daddr"
	}

	switch ep.Type {
	case EndpointAny, "":
		return []string{""} // no filter — matches everything

	case EndpointInternet:
		return []string{""} // handled by iifname/oifname in caller

	case EndpointGateway:
		return []string{fmt.Sprintf("%s %s", field, net.Gateway)}

	case EndpointNetwork:
		return []string{fmt.Sprintf("%s %s", field, net.Subnet)}

	case EndpointCIDR:
		if ep.Value == "" {
			return nil
		}
		return []string{fmt.Sprintf("%s %s", field, ep.Value)}

	case EndpointHost:
		// Host addresses are not tracked here; skip the rule.
		return nil

	case EndpointInstance:
		ip, ok := leaseIPs[ep.Value]
		if !ok {
			// Not in leases by ID; try by value as IP directly.
			return nil
		}
		return []string{fmt.Sprintf("%s %s", field, ip)}

	case EndpointLabel:
		key := fmt.Sprintf("%s=%s", ep.Key, ep.Value)
		ips := labelIPs[key]
		if len(ips) == 0 {
			return nil
		}
		return []string{fmt.Sprintf("%s { %s }", field, strings.Join(ips, ", "))}

	default:
		return nil
	}
}

// ruleDirs returns the nftables interface direction keyword(s) for a rule.
func ruleDirs(direction string, to Endpoint) []string {
	switch direction {
	case DirectionIngress:
		return []string{"oifname"}
	case DirectionEgress:
		return []string{"iifname"}
	case DirectionAny:
		return []string{"iifname", "oifname"}
	default: // forward
		if to.Type == EndpointInternet {
			return []string{"iifname"} // handled by oifname != bridge in caller
		}
		return []string{"iifname"}
	}
}

// ruleAction maps an ActionAllow/Deny/Reject to an nft verdict keyword.
func ruleAction(action string) string {
	switch action {
	case ActionAllow:
		return "accept"
	case ActionReject:
		return "reject"
	default: // deny
		return "drop"
	}
}

// protoPortExpr builds the protocol+port nft expression, e.g. "tcp dport 8080"
// or "tcp dport { 80, 443 }". Returns "" for any/all.
func protoPortExpr(proto string, ports []int) string {
	if proto == "" || proto == "any" {
		return ""
	}
	if proto == "icmp" {
		return "ip protocol icmp"
	}
	if len(ports) == 0 {
		return proto
	}
	if len(ports) == 1 {
		return fmt.Sprintf("%s dport %d", proto, ports[0])
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return fmt.Sprintf("%s dport { %s }", proto, strings.Join(parts, ", "))
}
