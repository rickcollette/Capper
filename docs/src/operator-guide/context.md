---
title: "Active context (org / account / project)"
description: "Set and inspect the active tenancy context the CLI sends with every request."
owner: "docs"
status: "stable"
reviewed: "2026-06-16"
outputs:
  - markdown
  - web
  - pdf
---

# Active context (org / account / project)

`capper context` sets the active **org / account / project** the CLI scopes
requests to — so you don't pass `--project` (and the tenancy headers) on every
command. See [Projects & tenancy](../concepts/projects.md) for the model.

## Commands

```bash
capper context show              # current org / account / project
capper context use-org <org>
capper context use-account <account>
capper context use-project <project>
capper context clear             # reset to defaults
```

## How it maps to requests

The active context populates the `X-Capper-Org-ID` / `X-Capper-Account-ID` /
`X-Capper-Project-ID` headers (CLI) or the equivalent SDK fields. The control
plane **validates these against your memberships** — you can only switch to a
tenant you belong to, so `use-account` on an account you're not a member of will
be rejected at the next request with a `403`. See the
[Security model](../concepts/security-model.md).

## Related

- [Projects & tenancy](../concepts/projects.md) · [Manage IAM](manage-iam.md)
  · [Security model](../concepts/security-model.md)
