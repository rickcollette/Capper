---
title: "Go SDK"
description: "Use the Capper Go SDK to drive the control plane."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Go SDK

The Go SDK (`capper/sdk/go`, package `cappersdk`) is a typed client for the
Capper REST API. It mirrors the API surface: roughly 40 grouped clients hanging
off a single `Client`, each method targeting a real `/api/v1/...` route.

## Quick start

```go
import cappersdk "capper/sdk/go"

c := cappersdk.New("http://127.0.0.1:8686", token)

instances, err := c.Instances.List(ctx, "default")
fn, err := c.Functions.Create(ctx, cappersdk.Function{Name: "echo", Runtime: "native", Command: []string{"/bin/cat"}})
res, err := c.Functions.Invoke(ctx, fn.ID, []byte("payload"))
```

## Multi-tenant context

Set the active organization, account, and project once; the SDK injects the
matching `X-Capper-*` headers on every request:

```go
c.UseOrg("org_acme").UseAccount("acct_prod").UseProject("proj_web")
```

## Groups

The client exposes grouped APIs including: `Instances`, `Images`, `Networks`,
`VPCs`, `Firewalls`, `LB`, `DNS`, `Ingress`, `IPAM`, `Storage`, `Databases`,
`Secrets`, `KMS`, `Certificates`, `Backups`, `BackupPolicies`, `IAM`, `Orgs`,
`Quotas`, `Governance`, `Realms`, `Regions`, `Zones`, `Nodes`, `NodePools`,
`Scheduler`, `Placement`, `Autoscale`, `Groups`, `InstanceTypes`, `GPU`,
`Migrations`, `CSD`, `Resources` (observability), `Functions`, `MCP`,
`Marketplace`, `Posture`, `AI`, `Health`, `Stacks`, `Queues`, `Search`,
`S3Creds`.

## Errors

Non-2xx responses return typed errors: `ErrNotFound`, `ErrForbidden`,
`ErrUnauthorized`, `ErrConflict`, and the general `*APIError` (with
`StatusCode` and `Message`). Use `errors.Is` / `errors.As`.

## Testing

The SDK test suite (`sdk/go/*_test.go`) spins up a real `api.Server` over an
in-memory store and exercises every group against it, including field-level
round-trips that assert the SDK structs match the API wire format. Run:

```bash
go test ./sdk/go/...
```
