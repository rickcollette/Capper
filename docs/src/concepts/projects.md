---
title: "Organizations, accounts, and projects"
description: "Capper's tenancy hierarchy and how resources are namespaced and isolated."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Organizations, accounts, and projects

Capper's tenancy is a three-level hierarchy:

```text
Organization  →  Account  →  Project
   (org_*)        (acct_*)     (default, …)
```

- **Organization** — the top of a tenant. Holds organization-level guardrails,
  managed policies, and org-root users. The local single-tenant deployment uses
  `org_local`.
- **Account** — a billing/isolation boundary inside an org (analogous to a cloud
  "account"). Holds IAM (users, groups, roles, policies), memberships, and the
  account's resources. The local deployment uses `acct_local`. Accounts can be
  `active` or `suspended` (a suspended account is denied at the API edge).
- **Project** — a resource namespace inside an account (default: `default`). Most
  resources live in a project; the `--project` flag / `X-Capper-Project-ID` header
  selects it.

## Identity and roots

Each level has **root users** that bypass account-level IAM (but never org
guardrails):

- **Org-root users** can administer the org and any account within it.
- **Account-root users** can administer their account.
- Ordinary principals (IAM users, IAM roles, service accounts) are governed by IAM
  policies and must be **members** of the account they act in.

See the [Security model](security-model.md) for how policies are evaluated and the
[Manage IAM](../operator-guide/manage-iam.md) guide for the commands.

## Selecting tenancy context

Every request carries an org/account/project context. From the CLI use `--project`
and the `org`/`project` commands (`capper org use-account`, `capper project
use-project`); over the API the `X-Capper-Org-ID` / `X-Capper-Account-ID` /
`X-Capper-Project-ID` headers select it. **These headers cannot widen scope** —
they are validated against the principal's memberships, so you can only act in a
tenant you belong to.

## Isolation

Authorization verifies that a resource's `account_id` matches the caller's resolved
account context (cross-account access requires explicit role assumption / trust).
This is the tenant-isolation boundary. Quotas and governance apply per project /
account — see [Quotas](../operator-guide/quotas.md) and
[Governance](../operator-guide/governance.md).

## Related commands

```bash
capper org create / list / use-account
capper project create / list / use-project
capper iam ...        # users, groups, roles, policies, assume-role, audit
capper quota ...      # per-project resource quotas
```

## Related

- [Security model](security-model.md) · [Manage IAM](../operator-guide/manage-iam.md)
  · [Quotas](../operator-guide/quotas.md)
