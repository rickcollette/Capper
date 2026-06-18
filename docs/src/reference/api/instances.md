---
title: "API reference — instances"
description: "Endpoints for listing, inspecting, and operating instances."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# API reference — instances

All paths are under `/api/v1` and require [authentication](overview.md).

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/instances` | list instances |
| `GET` | `/instances/{id}` | inspect an instance |
| `DELETE` | `/instances/{id}` | remove an instance |
| `GET` | `/instances/{id}/logs` | combined logs (`/logs/stdout`, `/logs/stderr` for streams) |
| `GET` | `/instances/{id}/events` | lifecycle events |
| `GET` | `/instances/{id}/metadata` | instance metadata (`/metadata/{tab}` for a section) |
| `GET` | `/instances/{id}/monitoring` | metrics/monitoring |
| `GET` | `/instances/{id}/terminal` | interactive terminal (WebSocket) |

Compute groups expose `GET /groups/{name}/instances`. Placement and autoscale have
their own route groups (`/placement/policies`, `/autoscale/policies`).

Responses use the [standard envelope](overview.md#response-envelope). See the
[Manage instances](../../operator-guide/manage-instances.md) guide for concepts and
the matching CLI verbs, and the [Go SDK](../sdk/go.md) `Instances` group.

## Related

- [API overview](overview.md) · [Manage instances](../../operator-guide/manage-instances.md)
