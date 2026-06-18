---
title: "API reference — IAM"
description: "Endpoints for users, groups, roles, policies, tokens, and audit."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# API reference — IAM

All paths are under `/api/v1` and require [authentication](overview.md). IAM has
both a current-account view (`/iam/...`) and an explicit account-scoped view
(`/accounts/{account}/iam/...`).

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/iam/users` | list users (current account) |
| `DELETE` | `/iam/users/{name}` | delete a user |
| `GET` | `/iam/groups` | list groups |
| `DELETE` | `/iam/groups/{group}/members/{user}` | remove a group member |
| `GET` | `/iam/roles` | list roles |
| `GET` | `/iam/policies` | list policies |
| `GET` | `/iam/tokens` | list tokens |
| `GET` | `/iam/audit` | query the audit log |
| `GET` | `/accounts/{account}/iam/users` | list users in a specific account |
| `GET`/`DELETE` | `/accounts/{account}/iam/{users,groups,roles,policies,service-accounts}/{id}` | account-scoped IAM |

Auth/session endpoints: `POST /auth/session` (exchange a token for a session +
CSRF), `DELETE /auth/session` (log out), `GET /auth/session` (session info).
Org/account root users are under `/orgs/{org}/root-users` and
`/orgs/{org}/accounts/{account}/root-users`.

Authorization follows the deny-by-default [security model](../../concepts/security-model.md).
The [Go SDK](../sdk/go.md) `IAM` group wraps these.

## Related

- [API overview](overview.md) · [Manage IAM](../../operator-guide/manage-iam.md)
  · [Security model](../../concepts/security-model.md)
