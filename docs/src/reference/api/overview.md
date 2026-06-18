---
title: "API reference — overview"
description: "Base path, authentication, the response envelope, CORS/CSRF, and conventions."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# API reference — overview

The REST API is served by `capper api start` at `/api/v1/...`. Start it with a
console and (for exposed deployments) TLS:

```bash
capper api start --listen 127.0.0.1:8686 \
  --tls-cert server.crt --tls-key server.key \
  --allowed-origin https://console.example.com
```

## Authentication

- **Bearer token** — `Authorization: Bearer <token>`. Issue tokens with
  `capper iam token`.
- **Session cookie** — POST a token to `/api/v1/auth/session` to exchange it for a
  `Secure`, `HttpOnly`, `SameSite=Strict` session cookie plus a CSRF token.

**Public (no auth):** `/api/v1/health`, `/api/v1/openapi.json`,
`/api/v1/version`, `/api/v1/daemon/status`, and `/api/v1/nodes/join` (join-token
auth).

## Tenancy headers

Select org/account/project context with `X-Capper-Org-ID`,
`X-Capper-Account-ID`, `X-Capper-Project-ID`. These are **validated against the
principal's memberships** — you can narrow to a tenant you belong to, never widen.
An unauthorized scope returns `403`.

## CSRF & CORS

- **CSRF:** cookie-authenticated state-changing requests must send the CSRF token
  (`X-CSRF-Token`) matching the CSRF cookie. Bearer-token requests are exempt.
- **CORS:** credentialed cross-origin access is allowlisted — loopback origins
  always, others via `--allowed-origin`.

## Response envelope

Responses use a JSON envelope:

```json
{ "data": { "...": "..." }, "error": null }
```

- `data` — the result payload (object or array), `null` on error.
- `error` — an error message string on failure, `null` on success.
- List endpoints may include pagination metadata.

Errors use standard HTTP status codes: `400` bad request, `401` unauthenticated,
`403` unauthorized/forbidden (incl. suspended account, disallowed tenant scope),
`404` not found, `405` method not allowed.

## OpenAPI

The live schema is served at `/api/v1/openapi.json`. Generate client code or
explore endpoints from it.

## Resource topics

For the **complete, source-generated list of every route**, see
[All routes](routes.md). Curated topic pages with context:
[Instances](instances.md) · [Networks](networks.md) · [IAM](iam.md) ·
[Storage](storage.md). Every subsystem in the
[operator guide](../../operator-guide/index.md) has a matching set of
`/api/v1/...` routes; the [Go SDK](../sdk/go.md) wraps all of them.

## Related

- [Go SDK](../sdk/go.md) · [CLI reference](../cli/capper.md)
  · [Security model](../../concepts/security-model.md)
