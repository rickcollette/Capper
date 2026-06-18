---
title: "Public IPAM: routable IP pools and Elastic IPs."
description: "Operate Capper's routable IP pools, reservations, and bindings."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Public IPAM

Capper's Public IPAM is its equivalent of AWS Elastic IPs: platform-owned
routable CIDR pools from which individual addresses are reserved, attached, and
bound to load balancers, VPC egress NAT, or passthrough hosts.

## Pools

A pool is a CIDR block with a declared set of allowed usage classes. Creating a
pool materializes its usable addresses, excluding the network, broadcast, and
gateway addresses.

```bash
capper ip-pool create public-main \
  --cidr 203.0.113.0/28 \
  --gateway 203.0.113.1 \
  --usage load-balancer,reserved,egress
capper ip-pool list
```

API: `POST /api/v1/ip-pools`, `GET /api/v1/ip-pools`. SDK: `c.IPAM`.

## Reservations

Reserve an address (auto or specific). Allocation rules are enforced: the pool
must be active, its usage must permit the requested purpose, reserved-only pools
require an explicit address, and exhaustion is reported.

```bash
capper ip reserve api-prod-ip --pool public-main --purpose load-balancer
capper ip reserve game-ip --pool public-main --address 203.0.113.10 --reserved
capper ip list --pool public-main
capper ip release api-prod-ip
```

API: `POST /api/v1/ips/reserve`, `GET /api/v1/ips`,
`POST /api/v1/ips/{id}/{release,attach,detach}`.

## Bindings

Attaching an address binds it to a target with a mode (`vip`, `snat`, `dnat`,
`passthrough`, `floating`). The same IP + protocol + external port cannot bind to
two targets; multiple ports on one IP to the same target are allowed.

In the Web UI: **Network → Routable IPs** (pools, reservations, and addresses).
