---
title: "Topology & nodes"
description: "Realms, regions, zones, nodes, placement, and the node lifecycle."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Topology & nodes

Physical placement is modeled as **realms → regions → zones → nodes**. Tenancy
(org/account/project) is orthogonal: tenants own resources, topology decides where
they run.

## The hierarchy

```bash
capper realm create / list / get / delete
capper region create / list / get / drain / evacuate
capper zone create / list / get / cordon / drain
```

- **Realm** — top-level failure/administrative domain.
- **Region** — a group of zones; supports `drain` and `evacuate` for maintenance.
- **Zone** — a failure domain inside a region; `cordon` stops new placements,
  `drain` moves work off.

## Nodes

Worker nodes run the `capper-agent` daemon and join the topology:

```bash
# on the node:
capper node join my-node --token <join-token> --address 10.0.0.5 --role compute
# on the control plane:
capper node approve my-node
capper node list
capper node cordon my-node     # stop new placements
capper node drain my-node      # move workloads off
capper node delete my-node
```

A node must be **approved** before it receives work. `register` records a node;
`cordon`/`drain` are the maintenance primitives.

## Placement

```bash
capper placement create / list / get / delete
capper scheduler capacity        # available capacity
capper scheduler simulate        # dry-run a placement decision
```

The scheduler places workloads by node roles, capacity, zone/region, and failure
domain. Use `scheduler simulate` to understand a decision before it happens.

## Maintenance pattern

`cordon` → `drain` → do the work → uncordon. For larger blast radius, `region`/
`zone` drain/evacuate move work at that level.

## Related

- [Control plane](../concepts/control-plane.md) · [Manage instances](manage-instances.md)
  · [Compute groups & autoscale](compute-groups.md)
