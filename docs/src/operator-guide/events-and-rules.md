---
title: "Events & event rules"
description: "View resource lifecycle events and react to them with event rules."
owner: "docs"
status: "stable"
reviewed: "2026-06-16"
outputs:
  - markdown
  - web
  - pdf
---

# Events & event rules

Every state change emits a **resource lifecycle event**. `capper event` views the
stream; `capper rule` reacts to it with **event rules** (event-driven automation).

## Viewing events

```bash
capper event list                # recent events (filter by resource/action)
capper event tail                # stream new events live
capper event export              # export the event log
```

Events carry the resource type/ID, the action (e.g. `instance.created`), the
principal, and a timestamp — the backbone of [observability](observability.md) and
audit.

## Event rules

```bash
capper rule create my-rule ...   # match events → trigger an action
capper rule list
capper rule delete my-rule
```

Rules let you wire reactions to events — e.g. fan a lifecycle event into a
[queue](queues.md), invoke a [function](serverless.md), or raise an
[alert](observability.md).

## Related

- [Observability](observability.md) · [Message queues](queues.md)
  · [Serverless (Functions & MCP)](serverless.md)
