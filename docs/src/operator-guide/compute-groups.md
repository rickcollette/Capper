---
title: "Compute groups & autoscale"
description: "Managed sets of instances that scale on policy."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Compute groups & autoscale

A **compute group** (`capper compute group`) is a managed set of instances that
the control plane keeps at a desired size and can scale automatically.

## Manage a group

A group launches from a **template** and is bounded by `--min`/`--max` with a
`--desired` starting size:

```bash
capper compute group create my-group --template web-tmpl --desired 3 --min 2 --max 10
capper compute group list
capper compute group inspect my-group
capper compute group scale my-group --desired 5    # manual scale to a new desired
capper compute group reconcile my-group            # converge actual → desired now
capper compute group delete my-group
```

| Flag (`group create`) | Purpose |
| --- | --- |
| `--template <name>` | launch template (required) |
| `--desired <n>` | starting replica count (default 1) |
| `--min <n>` / `--max <n>` | bounds the group (and autoscale) may operate within |

The control plane reconciles actual membership toward desired size, replacing
failed instances.

## Autoscale

Autoscale adjusts the desired size within the group's `--min`/`--max` bounds based
on a signal; decisions are recorded so you can audit why a scale happened.

```bash
capper compute group autoscale create my-group ...   # see `--help` for policy flags
capper compute group autoscale list
capper compute group autoscale inspect my-group
capper compute group autoscale delete my-group
```

Pair autoscale with [observability](observability.md) alerts to watch saturation.
See the [CLI reference](../reference/cli/capper.md#capper-compute-group) for the
full flag set.

## Placement

Group instances are placed by the scheduler across zones/nodes for spread and
failure-domain isolation — see [Topology & nodes](topology.md).

## Related

- [Manage instances](manage-instances.md) · [Topology & nodes](topology.md)
  · [Observability](observability.md) · [Scale a workload tutorial](../tutorials/scale-workload.md)
