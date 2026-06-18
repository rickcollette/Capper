---
title: "Go SDK reference"
description: "The cappersdk client, its resource groups, and authentication."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Go SDK reference

The Go SDK wraps the REST API. For a task-oriented walkthrough see the
[developer-guide SDK page](../../developer-guide/sdk.md); this page is the
reference for the client and its groups.

## Client

```go
import cappersdk "capper/sdk/go"

c := cappersdk.New("http://127.0.0.1:8686", "<bearer-token>")
insts, err := c.Instances.List(ctx)
```

`New(url, token)` returns a client that authenticates every call with the bearer
token. Tenancy context (org/account/project) is sent via the same `X-Capper-*`
headers the API validates — set them through the client where supported
(`c.UseOrg(...)`).

## Resource groups

The client exposes one group per subsystem. Available groups include:

`AI` · `Autoscale` · `Backups` · `BackupPolicies` · `Certificates` · `CSD` ·
`Databases` · `DNS` · `Firewalls` · `Functions` · `Governance` · `GPU` · `Groups`
· `Health` · `IAM` · `Images` · `Ingress` · `Instances` · `InstanceTypes` ·
`IPAM` · `KMS` · `LB` · `Marketplace` · `MCP` · `Migrations` · `Networks` ·
`NodePools` · `Nodes` · `Orgs` · `Placement` · `Posture` · `Queues` · `Quotas` ·
`Realms` · `Regions` · `Resources` · `Scheduler` · `Search` · `Secrets` ·
`Stacks` · `Storage` · `VPCs` · `Zones`.

Each group mirrors the corresponding `/api/v1/...` routes and the CLI verbs, so a
list/get/create/delete in the CLI maps to the same method on its SDK group.

## Errors

Methods return Go errors carrying the API's status and message (the
[response envelope's](../api/overview.md#response-envelope) `error` field). Check
and handle them as usual.

## Related

- [Developer-guide SDK walkthrough](../../developer-guide/sdk.md)
  · [API overview](../api/overview.md) · [CLI reference](../cli/capper.md)
