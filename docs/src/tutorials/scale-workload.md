---
title: "Scale a Workload"
description: "Create a compute group, configure autoscale, and attach a load balancer with health checks."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Scale a Workload

This tutorial takes a running single-instance application and turns it into a horizontally scaled fleet with automatic scaling and load-balanced traffic.

**Time:** 15 minutes
**Persona:** [SRE](../personas/sre.md)

---

## Step 1 — Create a Compute Group

Navigate to **Compute → Compute Groups** and click **Create Group**.

![Compute Groups](/assets/images/screenshots/14-compute-groups.png)

Fill in:
- **Name** — e.g. `myapp-fleet`
- **Capsule type** — the same type you used for your single instance
- **Desired** — start with `2` so you immediately have redundancy
- **Min** — `1` (never scale to zero)
- **Max** — `10` (cap blast radius during a traffic spike)

Once created, the group row shows a ± stepper for manual scaling. Capper will immediately provision the desired number of instances.

---

## Step 2 — Verify Instances

Expand the group row to see the instances it manages. Each entry shows the instance ID, its state, and the node it landed on.

Wait for all instances to reach the `running` state before proceeding.

---

## Step 3 — Attach a Load Balancer

Navigate to **Network → Load Balancers**.

![Load Balancers](/assets/images/screenshots/08-load-balancers.png)

Create a new load balancer or edit an existing one. Set the upstream selector to match the label that Capper assigns to instances in the group (the group name is used as a label by default).

With multiple upstreams registered, the LB begins round-robining requests across all healthy instances.

---

## Step 4 — Configure Health Checks

Navigate to **Network → Health Checks** and attach a probe to your load balancer's upstream pool.

Recommended settings for a stateless HTTP service:
- **Protocol** — `HTTP`
- **Path** — `/healthz`
- **Interval** — `10s`
- **Threshold** — `3` consecutive failures before removal; `2` before re-admission

The LB removes any instance that fails the probe and reinstates it automatically once it recovers — no manual intervention needed.

---

## Step 5 — Configure Ingress

Navigate to **Network → Ingress** to confirm your hostname points at the load balancer.

![Ingress](/assets/images/screenshots/09-ingress.png)

If you created an ingress rule in the [Deploy Your First Application](deploy-first-app.md) tutorial, update the upstream to point at the new load balancer rather than the single instance.

---

## Step 6 — Enable Autoscale

Back in **Compute → Compute Groups**, select your group and configure an autoscale policy:
- **Metric** — `cpu_percent` or `request_rate`
- **Scale-up threshold** — e.g. `70%` CPU for 60 seconds
- **Scale-down threshold** — e.g. `20%` CPU for 300 seconds
- **Cooldown** — `120s` between scale events

Capper evaluates the policy on every reconcile cycle and adjusts desired count within the min/max bounds you set. The Autoscale decisions endpoint (`GET /api/v1/groups/{name}/autoscale/decisions`) records every scaling decision for post-hoc analysis.

---

## What's Next

- [Harden Your Platform](harden-platform.md) — firewall the fleet, encrypt environment variables
- [Onboard a New Team](onboard-team.md) — give another team the same self-service capability
