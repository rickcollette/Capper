---
title: "Adding a module"
description: "The end-to-end pattern for adding a new subsystem across all four interfaces."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Adding a module

A new subsystem follows the same path the existing ones do: a sub-store in the
shared database, a manager, then exposure through the API, CLI, and SDK (and the
Web UI). Use an existing small subsystem (e.g. `internal/secret` or `internal/dns`)
as a template.

## Steps

1. **Types** — define your domain structs in `internal/<subsystem>/types.go`.
2. **Store** — add `internal/<subsystem>/store.go`: typed CRUD over a new table.
   Register the table's schema with the store's migrations in
   `internal/store/db.go` (use additive `CREATE TABLE IF NOT EXISTS` /
   `ALTER TABLE`). Wire the sub-store into `internal/store`.
3. **Manager** — add `manager.go` with business logic and validation; keep
   authorization decisions in the handler via the [authz engine](../concepts/security-model.md).
4. **API** — add handlers in `internal/api/handlers_<subsystem>.go` and register
   routes in `internal/api/server.go`. Build an `authz.AuthContext` and authorize
   before mutating. Return the standard response envelope.
5. **CLI** — add a cobra command group under `internal/cli` (see
   [Adding a CLI command](adding-a-cli-command.md)).
6. **SDK** — add a group to `sdk/go` mirroring the routes.
7. **Web UI** — add the corresponding views/calls in CapperWeb.
8. **Tests** — unit-test the store/manager on the in-memory backend; add API tests.

## Conventions

- One database, many sub-stores — do **not** open a second `sql.DB`.
- Parameterize all SQL (no string-built queries).
- Scope resources to org/account/project and enforce ownership in authz.
- Keep the four interfaces in sync: a new CRUD operation means CLI + API + SDK +
  Web UI all gain it.

## Validate

```bash
go build ./... && go vet ./... && go test ./...
make docs-inventory     # the module/route inventory should pick up your subsystem
```

## Related

- [Repository layout](repository-layout.md) · [Adding a CLI command](adding-a-cli-command.md)
  · [Security model](../concepts/security-model.md)
