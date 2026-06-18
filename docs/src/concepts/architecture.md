---
title: "Architecture"
description: "The control plane, nodes, store, and how requests flow through Capper."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Architecture

Capper is a single **control plane** that owns all platform state, plus any number
of **worker nodes** that run workloads and report to it. Everything an operator can
do is expressed as the same set of subsystem operations, surfaced identically
across the CLI, REST API, Go SDK, and Web UI.

## The pieces

- **Control-plane daemon** (`capper daemon`, or embedded via `api start
  --with-daemon`). Owns the database, runs the reconcilers/schedulers, and serves
  the REST API. It is the only writer of authoritative state.
- **Store.** One database holds all control-plane state, shared by ~45 sub-stores
  (one per subsystem). Default backend is embedded pure-Go SQLite (`~/.capper/capper.db`);
  the optional [CapDB backend](../operator-guide/capdb-backend.md) makes that same
  database a networked, pooled service.
- **Node agent** (`capper-agent`, `cmd/capper-agent`). Runs on each worker node:
  sends heartbeats, reports inventory, pushes host metrics, and supervises the
  services assigned to the node's roles. Nodes keep no SQL store.
- **Capsule runtime.** Executes `.cap` images via bwrap/chroot/crun/runc, with
  `capinit` as the in-capsule init. cgroups enforce resource limits.
- **REST API + Web console.** `capper api start` serves `/api/v1/...` and can serve
  the CapperWeb console as static assets.

## Request flow

1. A client (CLI/SDK/Web) calls the REST API with a bearer token or session cookie.
2. Middleware authenticates the principal, resolves tenancy
   (org/account/project), and applies CORS/CSRF rules.
3. The handler builds an `authz.AuthContext` and the
   [authorization engine](security-model.md) renders an allow/deny decision
   (deny-by-default).
4. On allow, the subsystem manager mutates state in its sub-store inside a
   transaction and emits resource events.
5. Reconcilers and the node agents converge actual state toward desired state
   (placement, networking programming, service supervision).

## Control loops

Capper is declarative where it can be: you describe desired state (an instance, a
load balancer, an IP binding) and reconcilers + node agents make it real, retrying
on failure. The [control-plane concept](control-plane.md) covers the daemon,
reconcilers, and scheduler in more depth.

## Multi-tenancy and topology

Tenancy is **organizations → accounts → projects** (see [Projects](projects.md));
physical placement is **realms → regions → zones → nodes**. The two are
orthogonal: tenants own logical resources, topology decides where they run. The
[Security model](security-model.md) explains how identity and isolation are
enforced across both.

## Related

- [Control plane](control-plane.md) · [Projects & tenancy](projects.md)
  · [Networking model](networking-model.md) · [Storage model](storage-model.md)
  · [Security model](security-model.md)
- Deep dive: [Architecture / System Overview](../architecture/system-overview.md)
