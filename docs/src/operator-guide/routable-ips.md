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

## Exclusions

An admin can *unlist* a routable address so the app stack never auto-allocates
it — for example, an address inside a routable subnet that is already in use by
the Capper Server Host. Excluding an address removes it from the allocatable set
and reconciles any already-materialized pool: an `available` address flips to
`excluded`, and a freshly created pool that contains the address skips it. A
global exclusion (no pool) applies to every pool whose CIDR contains the address;
a pool-scoped exclusion applies only within that pool.

An address that is already claimed (reserved, allocated, attached, or with live
bindings) is *not* silently pulled — the exclusion is refused until the address
is released or detached. Removing an exclusion returns the address to `available`.

```bash
capper ip-exclusion add 203.0.113.2 --reason "Capper Server Host"
capper ip-exclusion add 203.0.113.6 --pool public-main --reason "gateway HA"
capper ip-exclusion list
capper ip-exclusion remove ipexcl_<id>
```

API (admin only): `GET/POST /api/v1/admin/ip-exclusions`,
`DELETE /api/v1/admin/ip-exclusions/{id}`. SDK: `c.IPAM.{ListExclusions,AddExclusion,RemoveExclusion}`.

In the Web UI: **Admin → IP Exclusions** (admin-only). The Admin section also
surfaces **Routable IPs** and **Local Users** (platform operator accounts).
