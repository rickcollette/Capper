---
title: "Schedules"
description: "Cron-based schedules for recurring platform actions."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Schedules

`capper schedule` manages cron-based schedules that trigger recurring actions
(for example, invoking a [function](serverless.md), running a
[backup policy](manage-backups.md), or driving maintenance).

## Manage

```bash
capper schedule create my-job --cron '0 2 * * *' --action ...
capper schedule list
capper schedule delete my-job
```

## Cron syntax

Standard cron expressions (`min hour dom mon dow`), e.g. `0 2 * * *` for 02:00
daily. Keep scheduled actions idempotent so a missed/retried run is safe.

## Related

- [Serverless (Functions & MCP)](serverless.md) · [Manage backups](manage-backups.md)
  · [Message queues](queues.md)
