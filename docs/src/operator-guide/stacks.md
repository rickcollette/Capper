---
title: "Stacks (infrastructure as code)"
description: "Declare a set of resources and apply, diff, update, and destroy them as a unit."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Stacks (infrastructure as code)

`capper stack` manages **stacks** — a declared set of resources applied and
lifecycled together, with a plan/diff workflow.

## Workflow

```bash
capper stack plan ./stack.yaml       # preview changes
capper stack diff ./stack.yaml       # diff desired vs actual
capper stack apply ./stack.yaml      # create/update to desired state
capper stack update ./stack.yaml     # apply an updated definition
capper stack list
capper stack inspect <stack>
capper stack destroy <stack>         # tear down the whole stack
```

## Why stacks

Stacks give you repeatable, reviewable infrastructure: keep the definition in
version control, `plan`/`diff` in CI, and `apply` to converge. Destroying a stack
removes everything it created, in dependency order.

## Related

- [Architecture](../concepts/architecture.md) · [Operator guide index](index.md)
