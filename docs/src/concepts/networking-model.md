---
title: "Networking model"
description: "Virtual networks, VPCs, firewalls, load balancers, DNS, ingress, and public IPs."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Networking model

Capper networking is layered: private connectivity (networks and VPCs), traffic
control (security groups and firewalls), service exposure (load balancers, ingress,
DNS), and reachability from outside (public IPAM / elastic IPs).

## Private connectivity

- **Virtual networks** (`capper network`) — the basic L2/L3 connectivity an
  instance attaches to.
- **VPCs** (`capper vpc`) — isolated virtual private clouds composed of **subnets**,
  with **route tables**, internet gateways (`igw`), and NAT (`nat`). VPCs are the
  unit that [VPC Mobility](../operator-guide/vpc-mobility.md) migrates across
  realms/regions.

## Traffic control

- **Security groups** (`capper sg`) — instance-level allow rules attached to
  workloads within a VPC/subnet.
- **Firewalls** (`capper firewall`) — network policies programmed with **nftables**
  on the node. See [Firewall](../operator-guide/firewall.md).

The two compose: security groups express intent at the workload, firewalls enforce
policy at the node's packet path.

## Service exposure

- **Load balancers** (`capper lb`) — distribute traffic across instance backends;
  a load balancer can take its listen address from an allocated public IP.
- **Ingress** (`capper ingress`) — host/path routing rules in front of services.
- **DNS** (`capper dns`) — private zones, records, and service discovery; the
  resolver answers from the longest-matching zone. See
  [Manage DNS](../operator-guide/manage-dns.md).

## Reachability from outside

- **Public IPAM / Elastic IPs** (`capper ip`, `capper ip-pool`) — allocate routable
  IPs from pools and bind them to instances or load balancers. Node agents program
  the SNAT/DNAT/VIP rules. See [Public IPAM](../operator-guide/routable-ips.md).

## How a packet finds a workload

1. A client resolves a name via DNS (or hits a public/elastic IP).
2. Traffic lands on an ingress rule or a load balancer listener.
3. The LB/ingress selects a backend instance inside the VPC/subnet.
4. Security-group and firewall rules permit (or drop) the flow.
5. The instance, attached to its network/subnet, receives the packet.

## Related

- [Manage networks](../operator-guide/manage-networks.md)
  · [Firewall](../operator-guide/firewall.md)
  · [Load balancers](../operator-guide/load-balancers.md)
  · [Ingress](../operator-guide/ingress.md)
  · [Manage DNS](../operator-guide/manage-dns.md)
  · [Public IPAM](../operator-guide/routable-ips.md)
  · [VPC Mobility](../operator-guide/vpc-mobility.md)
