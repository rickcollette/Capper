---
title: "Quotas"
description: "Per-project resource quotas and limits."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Quotas

`capper quota` sets per-project resource quotas so one project cannot exhaust the
platform.

## Set and view

```bash
capper quota list                          # current quotas and usage
capper quota set --project <p> --resource instance --limit 50
```

| Flag (`quota set`) | Purpose |
| --- | --- |
| `--resource <type>` | resource type: `instance`, `storage`, or `network` |
| `--limit <n>` | the quota limit for that resource |

Quotas are enforced at admission: an operation that would exceed a project's limit
is denied. Combine with [governance](governance.md) guardrails for org-wide policy
and [IAM](manage-iam.md) for who-can-do-what.

## Common quota dimensions

Instances, volumes, buckets, public IPs, and other countable resources. Use
`capper quota list` to see the dimensions available in your deployment and current
consumption.

## Related

- [Governance](governance.md) · [Projects & tenancy](../concepts/projects.md)
  · [Manage IAM](manage-iam.md)
