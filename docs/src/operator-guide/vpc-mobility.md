---
title: "VPC Mobility"
description: "Migrate a VPC's workloads across realms/regions with plan → approve → execute → cutover."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# VPC Mobility

VPC Mobility migrates the workloads of a VPC across realms/regions through a
gated, four-phase lifecycle. It is exposed through the **REST API**, the **Go
SDK**, and the **Web UI** (the `vpcmover` subsystem); the `capper vpc` CLI group
covers VPC/subnet CRUD, while a mobility move is driven through those interfaces.

## The lifecycle

```text
plan  →  approve  →  execute  →  cutover
```

1. **Plan** — compute a migration plan for the source VPC: what moves, in what
   order, and the target realm/region. Review it before anything changes.
2. **Approve** — a human/authorized principal approves the plan. Nothing mutates
   until approval (this is the safety gate).
3. **Execute** — provision the target-side resources and replicate state while the
   source keeps serving.
4. **Cutover** — switch traffic to the target and decommission the source.

## Operating a move

- Drive it via the SDK (`c.VPCMobility…`) or the API (`/api/v1/...vpcmobility...`),
  or the Web console's VPC views.
- Each phase is observable; watch progress with [events](observability.md) and the
  plan/run status.
- Because the plan is approved before execution, you always review the blast radius
  first.

## Related

- [Manage networks & VPCs](manage-networks.md) · [Networking model](../concepts/networking-model.md)
  · [Topology & nodes](topology.md) · [Go SDK](../reference/sdk/go.md)
