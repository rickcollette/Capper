---
title: "Manage networks and VPCs"
description: "Create virtual networks and VPCs, attach instances, and manage subnets and routing."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Manage networks and VPCs

This guide covers private connectivity. For the conceptual layering (traffic
control, exposure, public reachability) see the
[Networking model](../concepts/networking-model.md).

## Virtual networks

```bash
capper network create my-net --mode nat --subnet 10.42.0.0/24 --dns
capper network list
capper network inspect my-net
capper network connect my-net --instance <id>      # attach an instance
capper network disconnect my-net --instance <id>
capper network delete my-net
```

| Flag (`network create`) | Purpose |
| --- | --- |
| `--mode nat\|isolated\|host-exposed` | connectivity mode (default `nat`) |
| `--subnet <cidr>` | subnet CIDR (default `10.42.0.0/24`) |
| `--dns` | auto-create a `.cap` DNS zone with gateway + records |

A virtual network is the basic connectivity an instance attaches to.

## VPCs, subnets, and routing

VPCs provide isolated private clouds composed of subnets with routing:

```bash
capper vpc create my-vpc --cidr 10.0.0.0/16 --home-region local --mobility manual
capper vpc subnet create my-vpc --cidr 10.0.1.0/24 --zone local-a
capper vpc route-table ...        # routes between subnets / gateways
capper vpc igw ...                # internet gateway
capper vpc nat ...                # NAT gateway
```

| Flag (`vpc create`) | Purpose |
| --- | --- |
| `--cidr <cidr>` | VPC CIDR block |
| `--home-region <slug>` | home region for the VPC |
| `--mobility manual\|...` | mobility policy (default `manual`); see [VPC Mobility](vpc-mobility.md) |
| `--name <display>` | display name |

Security groups (`capper sg`) attach allow-rules to workloads inside a VPC; node
firewalls (`capper firewall`) enforce network policy. See
[Firewall](firewall.md).

## Exposing services

Put a [load balancer](load-balancers.md) or [ingress](ingress.md) in front of
instances, give workloads names with [DNS](manage-dns.md), and make them reachable
from outside with [public IPs](routable-ips.md).

## Moving a VPC

To migrate a VPC's workloads across realms/regions, use the plan→approve→execute→
cutover flow in [VPC Mobility](vpc-mobility.md).

## Related

- [Networking model](../concepts/networking-model.md) · [Firewall](firewall.md)
  · [Load balancers](load-balancers.md) · [Ingress](ingress.md)
  · [Manage DNS](manage-dns.md) · [Public IPAM](routable-ips.md)
  · [VPC Mobility](vpc-mobility.md)
