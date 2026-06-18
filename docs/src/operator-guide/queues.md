---
title: "Message queues"
description: "Create queues and publish/consume messages."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Message queues

`capper queue` provides message queues for decoupling producers and consumers —
including driving [serverless functions](serverless.md) from queued events.

## Manage and use

```bash
capper queue create my-queue
capper queue list
capper queue publish my-queue --body '{"hello":"world"}'
capper queue consume my-queue
capper queue delete my-queue
```

## Patterns

- **Work distribution** — many consumers pull from one queue.
- **Event fan-out** — publish lifecycle/resource events for downstream processing.
- **Function triggers** — wire a queue to a [function](serverless.md) so messages
  invoke it.

## Related

- [Serverless (Functions & MCP)](serverless.md) · [Observability](observability.md)
