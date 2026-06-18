---
title: "Bottles (declarative app deployments)"
description: "Declare an application deployment and plan/deploy/remove it as a unit."
owner: "docs"
status: "stable"
reviewed: "2026-06-16"
outputs:
  - markdown
  - web
  - pdf
---

# Bottles (declarative app deployments)

`capper bottle` manages **Bottles** — declarative application deployments. You
describe the app (its instances, networking, and outputs) and Capper plans and
applies it as a unit, similar to [stacks](stacks.md) but app-shaped.

## Workflow

```bash
capper bottle validate ./app.bottle      # validate the definition
capper bottle plan ./app.bottle          # preview what will change
capper bottle deploy ./app.bottle        # create/update the deployment
capper bottle list
capper bottle deployments                # deployment history
capper bottle outputs my-app             # resolved outputs (URLs, IPs, …)
capper bottle import ...                 # import an existing definition
capper bottle remove my-app              # tear it down
```

## Bottles vs stacks

- **Bottle** — opinionated, application-centric: deploy an app and read its
  outputs.
- **[Stack](stacks.md)** — general infrastructure-as-code over arbitrary
  resources.

Use Bottles for app delivery, stacks for platform plumbing.

## Related

- [Stacks](stacks.md) · [Manage instances](manage-instances.md) · [Operator guide](index.md)
