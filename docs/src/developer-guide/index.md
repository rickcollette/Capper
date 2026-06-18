---
title: "Developer guide"
description: "Build, test, and extend Capper — repository layout, modules, CLI, and the SDK."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Developer guide

For contributors and integrators. If you only want to *operate* Capper, see the
[Operator guide](../operator-guide/index.md) instead.

## Start here

- [Repository layout](repository-layout.md) — where everything lives.
- [Build and test](build-and-test.md) — the `make` targets and the green-build
  invariant.
- [Adding a module](adding-a-module.md) — the pattern for a new subsystem
  (store → manager → API → CLI → SDK).
- [Adding a CLI command](adding-a-cli-command.md) — wiring a cobra command.
- [Go SDK](sdk.md) — using the client; mirrored by the
  [SDK reference](../reference/sdk/go.md).

## The golden invariant

Every change must keep the build green:

```bash
go build ./...
go vet ./...
go test ./...
cd ../CapperWeb && npm run build      # if the change touches the API/Web UI
```

When you change API CRUD, update the [Web UI](https://github.com/) (CapperWeb) and
the [SDK](sdk.md) to match — the four interfaces (CLI/API/SDK/Web) are expected to
stay in sync.

## Architecture context

Read the [Architecture concept](../concepts/architecture.md) and
[System overview](../architecture/system-overview.md) first — the subsystem pattern
(one sub-store per subsystem behind one database, reconcilers, node agent) is the
backbone every module follows.
