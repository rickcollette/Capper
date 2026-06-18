---
title: "Deploy Your First Application"
description: "Import an image, launch an instance, configure DNS, add a load balancer, and expose it via ingress."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Deploy Your First Application

This tutorial takes a `.cap` image and gets it publicly reachable through a load-balanced hostname in under 20 minutes.

**Time:** 20 minutes
**Personas:** [Application Developer](../personas/developer.md), [SRE](../personas/sre.md)

---

## Step 1 — Import an Image

Navigate to **Compute → Images** and click **Import**.

![Images](/assets/images/screenshots/06-images.png)

Upload your `.cap` file. Capper verifies the SHA-256 digest and extracts the image manifest. Once status shows **ready**, the image can be used to launch instances.

If you do not have a `.cap` image yet, build one with:

```bash
go run ./cmd/capper create myapp.cap examples/myapp/capper.json
```

---

## Step 2 — Launch an Instance

Navigate to **Compute → Instances** and click **Launch**.

![Instances](/assets/images/screenshots/02-instances.png)

Select your image, choose a capsule type (CPU/memory profile), and optionally:
- Attach a storage volume
- Set environment variables
- Target a specific availability zone

Click **Launch**. The instance moves through `created → starting → running`. If it stops immediately, check the log tail on the detail page.

---

## Step 3 — Set Up a Network

Your instance needs a network to be reachable from other services. Navigate to **Network → Networks**.

![Networks](/assets/images/screenshots/03-networks.png)

If your platform engineer has already created a VPC with a subnet, create a network that references that subnet. Otherwise create a flat network with a CIDR range that doesn't overlap existing networks.

---

## Step 4 — Configure DNS

Navigate to **Network → DNS** and create a zone for your domain.

![DNS](/assets/images/screenshots/07-dns.png)

Add an A record pointing your application hostname at the instance IP or the load balancer VIP you will create in the next step. Capper's internal resolver propagates the record immediately across the cluster.

---

## Step 5 — Create a Load Balancer

Navigate to **Network → Load Balancers** and click **Create LB**.

![Load Balancers](/assets/images/screenshots/08-load-balancers.png)

Configure:
- **Listen port** — the port the LB accepts traffic on (e.g. `443`)
- **Algorithm** — `round-robin` for stateless apps; `ip-hash` for session-affinity
- **Upstream selector** — a label that matches your instance

Attach a health check so the LB only routes to instances that are actually responding.

---

## Step 6 — Add an Ingress Rule

Navigate to **Network → Ingress** to expose the load balancer externally.

![Ingress](/assets/images/screenshots/09-ingress.png)

Set:
- **Hostname** — the public DNS name (must match your DNS record)
- **Upstream** — the load balancer created above
- **TLS** — attach a certificate for HTTPS termination

Once the ingress rule is active, `curl https://your-hostname/` should return your application's response.

---

## Step 7 — Deploy as a Stack

For repeatable deployments, encode everything above in a Stack definition. Navigate to **Platform → Stacks**.

![Stacks](/assets/images/screenshots/17-stacks.png)

A Stack (Bottle) is a YAML/JSON template declaring networks, instances, load balancers, and DNS as a unit. Apply the same template to deploy identical staging and production environments with a single API call:

```bash
curl -X POST /api/v1/stacks \
  -H "Content-Type: application/json" \
  -d @my-stack.json
```

---

## What's Next

- [Scale a Workload](scale-workload.md) — add a compute group and autoscale policy
- [Harden Your Platform](harden-platform.md) — firewall the network, encrypt secrets with KMS
