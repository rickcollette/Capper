---
title: "Operational jobs"
description: "Define, run, and inspect one-off and on-demand operational jobs."
owner: "docs"
status: "stable"
reviewed: "2026-06-16"
outputs:
  - markdown
  - web
  - pdf
---

# Operational jobs

`capper job` manages operational **jobs** — discrete units of work you run on
demand (migrations, maintenance tasks, batch operations), as opposed to long-lived
[instances](manage-instances.md) or recurring [schedules](schedules.md).

## Commands

```bash
capper job create my-job ...    # define a job
capper job run my-job           # run it now
capper job list
capper job logs my-job          # output from a run
capper job delete my-job
```

## Jobs vs schedules vs functions

- **Job** — run-to-completion operational task, invoked on demand.
- **[Schedule](schedules.md)** — runs an action on a cron cadence.
- **[Function](serverless.md)** — event/trigger-driven serverless code.

Pair a job with a [schedule](schedules.md) to run it recurringly.

## Related

- [Schedules](schedules.md) · [Serverless (Functions & MCP)](serverless.md)
