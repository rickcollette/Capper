---
title: "Control plane"
description: "The daemon, reconcilers, scheduler, and how desired state is converged."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Control plane

The control plane is the authoritative brain of a Capper deployment. It is a
single daemon that owns the database, exposes the REST API, and runs the loops
that turn desired state into reality.

## Running it

- **Embedded (dev):** `capper api start --with-daemon` runs the API and the daemon
  in one process. `make capper-run` wraps this into a runnable bundle.
- **Separate daemon:** `capper daemon` runs the control loops; `capper api start`
  serves the API against the same store. Use this split for production.
- **All-in-one:** `capper aio up` provisions and supervises the API, daemon, and
  local services on a single node.

Check health any time with `capper status` (daemon + subsystem status) and the
public `/api/v1/health`, `/api/v1/daemon/status` endpoints.

## What it owns

- **State.** All subsystem records live in one database, behind ~45 sub-stores.
  Writes are transactional; reads run concurrently. See the
  [CapDB backend](../operator-guide/capdb-backend.md) for the networked option.
- **Identity & authorization.** Token issuance/verification (HMAC-signed bearer
  tokens), IAM policy evaluation, and the deny-by-default
  [authorization engine](security-model.md).
- **Reconcilers.** Per-subsystem loops that converge actual state toward desired
  state — placement of instances, programming of networking, certificate renewal,
  autoscale decisions, and more — retrying on failure.
- **Scheduler / placement.** Chooses which node runs a workload based on topology
  (realm/region/zone), node roles, capacity, and failure domains.
- **Events & observability.** Resource lifecycle events, drift detection, metrics,
  and alert evaluation (see [Observability](../operator-guide/observability.md)).

## Desired vs actual state

You declare *what* you want; the control plane decides *how* and *where*. A request
records desired state in the store and emits an event; reconcilers and the node
agents then make actual state match, idempotently. This is why most operations
return quickly and converge asynchronously — watch progress with
`capper event` and `capper status`.

## Nodes and the agent

Worker nodes do not touch the database. The `capper-agent` daemon on each node
pulls its assignments, supervises the services for its roles, heartbeats, and
reports inventory and host metrics back to the control plane. A node joins with
`capper node join` and must be approved (`capper node approve`) before it receives
work. See [Topology & nodes](../operator-guide/topology.md).

## High availability

Today the control plane and its database are a single logical unit. With the
embedded backend that unit is one process; with CapDB it is one networked server
reachable by multiple control-plane processes. Replicated/HA CapDB is a roadmap
item — see the
[CapDB availability posture](../operator-guide/capdb-backend.md#availability-posture).

## Related

- [Architecture](architecture.md) · [Projects & tenancy](projects.md)
  · [Security model](security-model.md)
