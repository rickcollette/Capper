---
title: "AI agents & MCP"
description: "Manage AI agents, sessions, and Model Context Protocol (MCP) servers with per-tool IAM."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# AI agents & MCP

`capper ai` manages AI agents, sessions, and **MCP** (Model Context Protocol)
servers. Managed MCP servers expose tools to agents with **per-tool IAM** and
**approval gates**.

## Agents and sessions

```bash
capper ai agent ...        # define/manage agents
capper ai session ...      # manage agent sessions
capper ai mcp ...          # MCP servers from the ai group
```

## MCP servers

```bash
capper mcp list
capper mcp deploy ...       # deploy a managed MCP server
capper mcp tools <server>   # list the tools it exposes
capper mcp approvals ...    # review/approve gated tool calls
```

## Security model for tools

- **Per-tool IAM** — each MCP tool is an authorizable action; grant least
  privilege via [IAM](manage-iam.md).
- **Approval gates** — sensitive tool calls require explicit approval
  (`capper mcp approvals`) before they execute.

This keeps agent-driven actions inside the same deny-by-default
[authorization model](../concepts/security-model.md) as everything else in Capper.

## Related

- [Serverless (Functions & MCP)](serverless.md) · [Manage IAM](manage-iam.md)
  · [Security model](../concepts/security-model.md)
