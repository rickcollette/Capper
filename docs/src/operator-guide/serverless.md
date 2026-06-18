---
title: "Serverless: Functions and MCP servers."
description: "Operate Capper Functions and managed MCP tool servers."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Serverless

Capper's serverless layer has two tracks that share one execution, IAM, and
audit foundation: **Functions** (Lambda-style) and **MCP servers** (managed
Model Context Protocol tool servers with a stricter security contract).

## Functions

A function is a versioned unit of code invoked synchronously (HTTP/manual) or
asynchronously via event triggers (queue, schedule). Every invocation is
recorded with status, duration, and result.

```bash
capper fn create resize --runtime native --command /usr/local/bin/resize
capper fn invoke resize --payload '{"width":128}'
capper fn invocations resize
capper fn delete resize
```

API: `/api/v1/functions` (CRUD), `POST /api/v1/functions/{id}/invoke`,
`/api/v1/functions/{id}/{versions,triggers,invocations}`. SDK: `c.Functions`.

Triggers bind an event source to a function; the event router fans a
`(type, source)` event out to every enabled trigger.

## MCP servers

MCP servers expose tools that AI agents can call, so each tool carries an IAM
action and optional approval gating. `InvokeTool` enforces, in order:

1. the tool must exist and be enabled;
2. the caller must pass the tool's IAM action check;
3. dangerous / approval-required tools (per the server's approval policy:
   `none`, `dangerous-only`, `all`) open a **pending approval** instead of
   executing.

```bash
capper mcp deploy admin-tools --runtime mcp-go --approval-policy dangerous-only
capper mcp tools list admin-tools
capper mcp approvals            # pending tool-call approvals
```

API: `/api/v1/mcp/servers` (CRUD), `POST /api/v1/mcp/servers/{id}/tools/sync`,
`POST /api/v1/mcp/servers/{id}/tools/{tool}/invoke`,
`GET /api/v1/mcp/approvals`, `POST /api/v1/mcp/approvals/{id}/{approve,deny}`.
SDK: `c.MCP`.

In the Web UI: **Serverless → Functions** and **Serverless → MCP Servers**
(which surfaces the pending-approval queue).
