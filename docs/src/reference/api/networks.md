---
title: "API reference — networks"
description: "Endpoints for virtual networks (and related VPC routes)."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# API reference — networks

All paths are under `/api/v1` and require [authentication](overview.md).

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/networks` | list virtual networks |
| `POST` | `/networks` | create a network |
| `GET` | `/networks/{name}` | inspect a network |
| `DELETE` | `/networks/{name}` | delete a network |
| `GET` | `/networks/{id}/monitoring` | network metrics |

VPCs, subnets, route tables, security groups, gateways, load balancers, ingress,
DNS, and public IPAM each have their own `/api/v1/...` route groups, mirroring the
[networking model](../../concepts/networking-model.md). The
[Go SDK](../sdk/go.md) wraps them via the `Networks`, `VPCs`, `Firewalls`, `LB`,
`Ingress`, `DNS`, and `IPAM` groups.

## Related

- [API overview](overview.md) · [Manage networks & VPCs](../../operator-guide/manage-networks.md)
  · [Networking model](../../concepts/networking-model.md)
