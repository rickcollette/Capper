---
title: "Manage IAM"
description: "Users, groups, roles, policies, tokens, assume-role, and the audit log."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Manage IAM

`capper iam` administers identities and access within an account. For the
evaluation model (deny-by-default, guardrails, ownership) see the
[Security model](../concepts/security-model.md).

## Principals

```bash
capper iam user create alice --local-user alice   # IAM user (optionally bound to an OS user)
capper iam group ...              # groups
capper iam role ...               # roles (assumable)
capper iam service-account ...    # non-human principals
capper iam whoami                 # the current principal
```

### A worked example: scoped service account

```bash
# 1. Create a service account for a CI pipeline.
capper iam service-account create ci-deployer

# 2. Attach a least-privilege policy (allow deploy actions only).
capper iam policy create ci-deploy --document ./ci-deploy-policy.json
capper iam grant --principal ci-deployer --policy ci-deploy

# 3. Issue a bearer token for it and use it.
capper iam token ...              # see `capper iam token --help`
export CAPPER_TOKEN=<issued-token>
curl -H "Authorization: Bearer $CAPPER_TOKEN" https://capper.example/api/v1/instances
```

Revoke the token the moment it is no longer needed; revocation takes effect within
seconds.

## Policies

Policies are JSON documents of statements (`effect` allow/deny, `actions`,
`resources`). Explicit deny always wins; at least one matching allow is required.

```bash
capper iam policy ...            # create/list/attach/detach policies
capper iam grant ...             # grant access
```

Org-level guardrails (managed at the org) can deny actions no policy or root can
override.

## Tokens

```bash
capper iam token ...             # issue / list / revoke bearer tokens
```

Tokens are HMAC-signed and revocable. Revoking a token takes effect within
seconds (a short-TTL cache fronts the revocation check). Send a token as
`Authorization: Bearer <token>` to the API.

## Assume-role and cross-account

```bash
capper iam assume-role ...       # assume a role (cross-account via trust)
capper iam cross-account ...     # manage cross-account trust
```

Cross-account access is explicit: the target account must trust the assuming
principal, and authorization records the acting account.

## Audit

```bash
capper iam audit                 # query authorization/audit events
capper audit ...                 # account/resource-scoped audit log
```

## Roots and membership

Org-root and account-root users bypass account IAM (but not guardrails). Ordinary
principals must be **members** of the account they act in; tenant-scope request
headers cannot widen scope. See [Projects & tenancy](../concepts/projects.md).

## Related

- [Security model](../concepts/security-model.md) · [Projects & tenancy](../concepts/projects.md)
  · [Governance](governance.md) · [Quotas](quotas.md)
