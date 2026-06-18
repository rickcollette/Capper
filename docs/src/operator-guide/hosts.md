---
title: "Host inventory"
description: "Inventory hosts, run capability checks, and label them."
owner: "docs"
status: "stable"
reviewed: "2026-06-16"
outputs:
  - markdown
  - web
  - pdf
---

# Host inventory

`capper host` inventories the physical/virtual hosts available to the platform and
checks their capabilities (the runtime backends, namespaces, and tools capsules
need).

## Commands

```bash
capper host doctor              # capability + dependency report for this host
capper host list                # inventoried hosts
capper host inspect <host>      # details
capper host register ...        # register a host
capper host label <host> key=value
```

## `host doctor`

Run `capper host doctor` before bringing a host into service — it reports whether
Bubblewrap / user namespaces / `crun` / `runc` are available and flags anything
missing that capsules or networking depend on. It's also the first command to run
when capsules fail to start (see [Troubleshooting](../getting-started/troubleshooting.md)).

## Hosts vs nodes

- **Host** — a machine and its raw capabilities.
- **[Node](topology.md)** — a host that has joined the topology and runs the
  `capper-agent`, eligible for placement.

## Related

- [Topology & nodes](topology.md) · [Manage instances](manage-instances.md)
  · [Troubleshooting](../getting-started/troubleshooting.md)
