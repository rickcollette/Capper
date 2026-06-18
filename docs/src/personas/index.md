---
title: "Personas"
description: "Who uses Capper and what they care about."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Personas

Capper serves several distinct roles. Understanding which persona matches your
work will point you to the right guides, workflows, and reference pages.

## Platform Engineer

Responsible for standing up and maintaining the Capper control plane itself.
Manages physical nodes, the topology layer (realms / regions / zones), VPC
networking, IAM bootstrap, and system-level quotas.

**Primary surfaces:** Topology, VPCs, IAM, Quotas, Settings
**Key concern:** Availability, resource utilisation, security posture of the platform itself.

[Read the Platform Engineer guide](platform-engineer.md)

## Application Developer

Builds and ships applications as capsule images. Launches instances, wires up
DNS, requests storage, and publishes stacks.

**Primary surfaces:** Instances, Images, Stacks, DNS, Storage, Certificates
**Key concern:** Fast iteration, self-service deployment, clear error messages.

[Read the Application Developer guide](developer.md)

## Security Engineer

Audits access, hardens policies, manages encryption keys, inspects firewall
rules, and reviews the posture scanner output.

**Primary surfaces:** IAM, KMS, Firewalls, Posture, Audit Log
**Key concern:** Least privilege, auditability, key rotation, incident response.

[Read the Security Engineer guide](security-engineer.md)

## Database Administrator

Manages Capper-hosted databases, schedules backups, monitors replication lag,
and handles point-in-time restores.

**Primary surfaces:** Databases, Backups, Storage
**Key concern:** RPO/RTO, backup integrity, connection reliability.

[Read the DBA guide](dba.md)

## DevOps / SRE

Deploys application stacks, configures load balancers and health checks, wires
up ingress, tunes autoscaling, and responds to alerts.

**Primary surfaces:** Stacks, Load Balancers, Health Checks, Compute Groups, Ingress
**Key concern:** Zero-downtime deployments, traffic routing, observability.

[Read the SRE guide](sre.md)
