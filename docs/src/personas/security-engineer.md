---
title: "Security Engineer"
description: "Guide for security engineers hardening and auditing Capper."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Security Engineer

Your job is to ensure the platform is correctly locked down, auditable, and
recoverable after an incident. Capper gives you IAM policy simulation, KMS key
management, firewall rule inspection, posture scanning, and an immutable audit
trail.

## IAM — Policy Simulation

Before granting a permission, use the Simulate tab to confirm exactly which
actions a principal can take against a given resource. The simulator evaluates
all attached policies and returns the decision with the matching rule.

![IAM Policy Simulator](/assets/images/screenshots/30-iam-policy-sim.png)

## KMS — Encryption Key Management

All sensitive workload data should be encrypted with a Capper KMS key. The KMS
page lets you create keys, rotate them on demand, and test encrypt / decrypt
operations directly in the console.

![KMS / Secrets](/assets/images/screenshots/12-secrets.png)

**Key rotation:** click **Rotate** on any key to generate a new key version.
Old ciphertext remains decryptable via the previous version; new encrypt calls
use the latest version.

## Firewalls — Network Policy

Inspect and manage firewall rules per network. Rules are evaluated in priority
order; the first match wins.

![Firewalls](/assets/images/screenshots/10-firewall.png)

Each firewall row expands to show its rules. Use the inline form to add
rules specifying direction (inbound/outbound), protocol, port range, source
CIDR, and action (allow/deny).

## Posture — Security Findings

The posture scanner runs continuously and surfaces misconfigurations, over-
privileged policies, open firewall rules, and unencrypted volumes.

![Posture](/assets/images/screenshots/20-posture.png)

Each finding includes a severity level, a description, and a recommended
remediation step.

## Audit Log

Every state-changing API call is appended to an immutable audit log. Filter by
user, resource type, or time window to reconstruct the sequence of events
leading to an incident.

The IAM section's Audit Log entry in the left nav surfaces the raw event
stream. The dashboard also shows the last three events inline.

## Key Workflows

- [Harden Your Platform](../tutorials/harden-platform.md) — KMS → Firewalls → IAM Policies → Posture → Audit review
- [Rotate All KMS Keys](../operator-guide/manage-iam.md) — key rotation runbook
