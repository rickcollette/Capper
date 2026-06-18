---
title: "Firewall"
description: "Network firewall policies enforced with nftables on the node."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Firewall

`capper firewall` manages network firewall policies, programmed with **nftables**
on the node. Firewalls enforce policy in the packet path; security groups
(`capper sg`) express allow-intent at the workload — the two compose.

## Rules and policy

```bash
capper firewall init                 # initialise the firewall on a node
capper firewall rule ...             # add/manage rules
capper firewall list                 # list policies/rules
capper firewall inspect <policy>
capper firewall apply                # program the current policy via nftables
capper firewall delete <policy>
capper firewall reset                # clear programmed rules
```

## Workflow

1. Define rules (`firewall rule ...`).
2. Review with `firewall list` / `inspect`.
3. `firewall apply` to program nftables on the node.
4. `firewall reset` to roll back to a clean state.

Treat firewall changes like any production network change: review, apply to a
canary node, then roll out.

## Related

- [Networking model](../concepts/networking-model.md) · [Manage networks & VPCs](manage-networks.md)
