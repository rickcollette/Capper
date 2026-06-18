---
title: "Harden Your Platform"
description: "Create KMS keys, tighten firewall rules, simulate IAM policies, run a posture scan, and review the audit log."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Harden Your Platform

This tutorial walks a security engineer through a full hardening pass: encryption at rest, network access control, least-privilege IAM, automated posture scanning, and audit trail review.

**Time:** 30 minutes
**Persona:** [Security Engineer](../personas/security-engineer.md)

---

## Step 1 — Create KMS Encryption Keys

Navigate to **Platform → KMS**.

![KMS / Secrets](/assets/images/screenshots/12-secrets.png)

Create a key for each sensitivity tier your organisation uses — for example:
- `infra-secrets` (AES-256-GCM) — for infrastructure credentials
- `app-data` (AES-256-GCM) — for application-level encryption
- `signing` (ECDSA-P256) — for image signing and JWT verification

For each key, click the **Encrypt / Decrypt** toggle to verify the key is operational before depending on it.

**Key rotation schedule:** Click **Rotate** on each key at least quarterly. Old ciphertext remains decryptable; new encrypt calls automatically use the latest key version.

---

## Step 2 — Audit Firewall Rules

Navigate to **Network → Firewalls**.

![Firewalls](/assets/images/screenshots/10-firewall.png)

Expand each firewall and review its rules. For every inbound `allow` rule, ask:
- Is this source CIDR as narrow as it can be?
- Is this port range as narrow as it can be?
- Does anything still need `any` as the protocol?

Add an explicit `deny all` rule at the lowest priority (highest priority number) to make the default posture clear. Remove any rules that were created for debugging and never cleaned up.

---

## Step 3 — Simulate IAM Policies

Navigate to **IAM → Simulate**.

![IAM Policy Simulator](/assets/images/screenshots/30-iam-policy-sim.png)

For each non-admin group, test that:
- The group **can** perform the actions it needs (e.g. `instance:create`)
- The group **cannot** perform admin actions (e.g. `iam:policy:create`)
- Cross-tenant resource access returns `deny`

If the simulator returns `allow` for something it shouldn't, find the offending policy in **IAM → Policies** and tighten the resource scope or remove the action.

---

## Step 4 — Review IAM Users and Tokens

Navigate to **IAM → Users**.

![IAM Users](/assets/images/screenshots/11-iam.png)

Check for:
- Users with no group membership (orphaned accounts)
- Service accounts with human names (or vice versa)
- Tokens that have not been rotated in over 90 days (**IAM → Tokens**)

Revoke any token that is no longer in active use. Tokens cannot be un-revoked.

---

## Step 5 — Run a Posture Scan

Navigate to **Platform → Posture** and click **Run Scan**.

![Posture](/assets/images/screenshots/20-posture.png)

The scanner checks:
- Unencrypted block volumes
- Open firewall rules (`0.0.0.0/0` inbound allow)
- Over-privileged IAM policies (wildcards on resource scope)
- Instances with no health check attached
- KMS keys older than 90 days without a rotation event

Address every **critical** and **high** finding before proceeding. Document **medium** findings with a planned remediation date.

---

## Step 6 — Review the Audit Log

Navigate to **IAM → Audit Log**.

Filter to the last 24 hours and look for:
- Any `DELETE` actions on IAM resources you did not perform
- `policy:create` or `policy:attach` actions by non-admin users
- Repeated `403` responses (potential probing)
- Actions at unusual hours

The audit log is append-only. Every API call that mutates state produces an entry with the principal, action, resource, timestamp, and source IP.

---

## Hardening Checklist

- Bootstrap admin token rotated or revoked after initial setup
- At least one KMS key per sensitivity tier, all rotated within 90 days
- No firewall rule with `0.0.0.0/0` inbound allow on non-public services
- All non-admin groups pass policy simulation for least privilege
- Zero critical posture findings
- Audit log reviewed and no unexplained anomalies

---

## What's Next

- Schedule a monthly posture scan and audit log review
- [Onboard a New Team](onboard-team.md) — apply these controls to new tenants from day one
