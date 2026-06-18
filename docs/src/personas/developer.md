---
title: "Application Developer"
description: "Guide for developers deploying applications on Capper."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Application Developer

You package your application as a `.cap` image and use Capper to run it. The
control plane gives you self-service access to instances, storage, DNS, and
TLS — without opening a ticket.

## Images

Upload your `.cap` image through the console or `POST /api/v1/images/import`.
The Images page shows import status, file size, and the SHA-256 digest used for
provenance verification.

![Images](/assets/images/screenshots/06-images.png)

## Instances

Launch an instance from any imported image. The **Launch** form lets you pick
a capsule type, attach storage volumes, set environment variables, and target
a specific availability zone.

![Instances](/assets/images/screenshots/02-instances.png)

After launch the detail view shows live status, log tail, metadata, and
attached resources.

## Stacks — Multi-Resource Deployments

A stack (also called a Bottle) is a declarative template that provisions
networks, instances, load balancers, and DNS records as a unit. Apply the same
stack definition to create identical environments for staging and production.

![Stacks](/assets/images/screenshots/17-stacks.png)

## DNS

Register a DNS zone and add records to give your instance a stable hostname.
Capper's internal DNS resolver makes records available immediately within the
private network.

![DNS](/assets/images/screenshots/07-dns.png)

## Certificates

Request and manage TLS certificates. Certificates are attached to ingress
rules and load balancers to terminate HTTPS at the edge.

## Storage — Buckets and Volumes

Create S3-compatible object buckets for static assets and backups, or attach
block volumes directly to an instance.

![Storage](/assets/images/screenshots/05-storage.png)

## Ingress

Expose an instance to external traffic with an ingress rule. Set the hostname,
upstream port, and optional TLS termination.

![Ingress](/assets/images/screenshots/09-ingress.png)

## Key Workflows

- [Deploy Your First Application](../tutorials/deploy-first-app.md) — import image → launch instance → DNS → LB
- [Create a Stack](../tutorials/deploy-first-app.md) — declarative multi-resource deployment
