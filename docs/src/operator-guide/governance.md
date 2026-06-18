---
title: "Governance"
description: "Organization-level guardrails that constrain what any account can do."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Governance

`capper governance` manages organization-level **guardrails** — policy that
applies across all accounts in an org and that even root users cannot override.

## Guardrails

```bash
capper governance add ...        # define a guardrail (e.g. deny an action org-wide)
capper governance list
capper governance eval ...       # evaluate what a guardrail would allow/deny
```

In the [authorization pipeline](../concepts/security-model.md), an org guardrail
with an explicit **deny** wins over any account policy and over org/account-root
bypass. Use guardrails for non-negotiable controls (e.g. "no public buckets",
"region restrictions").

## Governance vs IAM vs quotas

- **Guardrails (governance)** — org-wide hard limits, deny-wins, not overridable.
- **IAM policies** — per-account allow/deny for principals.
- **Quotas** — per-project resource counts.

Use them together: guardrails for the floor, IAM for delegation, quotas for
capacity.

## Related

- [Security model](../concepts/security-model.md) · [Manage IAM](manage-iam.md)
  · [Quotas](quotas.md)
