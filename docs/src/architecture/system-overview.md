---
title: "System overview"
description: "Components, data flow, the store, and the reconcile loops in detail."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# System overview

```text
                         ┌─────────────────────────────────────────┐
   CLI ─┐                │            Control plane                  │
   SDK ─┤── REST /api/v1 │  api (handlers, middleware, authz)        │
   Web ─┘     ▲          │  controller → managers → sub-stores       │
              │          │  control (daemon: reconcilers, scheduler) │
              │          │  store: ONE database (~45 sub-stores)     │
              │          └───────────────▲───────────────┬──────────┘
              │                          events           │ assignments
              │                                            ▼
        capper-agent (per node): heartbeat · inventory · metrics · supervise
                                            │
                                  capsule runtime (bwrap/chroot/crun/runc)
```

## Components

- **API layer** (`internal/api`) — HTTP server, auth/CORS/CSRF middleware, per-
  subsystem handlers. Builds an `authz.AuthContext` and authorizes every mutating
  request.
- **Controller** (`internal/controller`) — binds the store and subsystem managers
  for handlers and the CLI.
- **Daemon** (`internal/control`) — runs reconcilers, the placement scheduler, and
  service supervision; the only component that drives convergence.
- **Store** (`internal/store`) — one database, ~45 typed sub-stores, additive
  startup migrations. Default pure-Go SQLite; optional networked
  [CapDB](../operator-guide/capdb-backend.md).
- **Node agent** (`cmd/capper-agent`) — pulls assignments, supervises services for
  its roles, heartbeats, reports inventory + host metrics. Holds no SQL state.

## Data flow

1. A client calls `/api/v1/...` with a bearer token or session cookie.
2. Middleware authenticates, resolves tenancy, applies CORS/CSRF.
3. The handler authorizes (deny-by-default) and mutates the relevant sub-store in
   a transaction, emitting a resource event.
4. Reconcilers + node agents converge actual state to desired state, idempotently.

## Why one database

A single database shared by typed sub-stores keeps tenancy, transactions, and
backups simple, and maps cleanly onto the networked CapDB service when you need
multiple control-plane processes. There is no per-subsystem database sprawl.

## Trust boundaries

The API edge (auth, tenant-scope validation, CORS/TLS), the deny-by-default
authorization pipeline, and per-account resource ownership are the security
boundaries — see the [Security model](../concepts/security-model.md). Node agents
are semi-trusted: they execute assignments but never touch authoritative state
directly.

## Related

- [Architecture concept](../concepts/architecture.md) · [Control plane](../concepts/control-plane.md)
  · [Security model](../concepts/security-model.md) · [CapDB backend](../operator-guide/capdb-backend.md)
