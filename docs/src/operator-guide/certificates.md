---
title: "Certificates"
description: "Issue and renew TLS certificates from the internal CA or ACME/Let's Encrypt."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Certificates

`capper cert` manages TLS certificates signed by Capper's local CA (and supports
ACME / Let's Encrypt issuance with a renewal scheduler and bindings).

## Issue and list

```bash
capper cert ca                 # manage / inspect the internal CA
capper cert issue web --cn web.internal.example --dns web.internal.example --dns www.internal.example
capper cert list
capper cert revoke <cert-id>
```

| Flag (`cert issue`) | Purpose |
| --- | --- |
| `--cn <name>` | certificate common name (defaults to the cert NAME) |
| `--dns <name>` | a DNS SAN entry; repeatable for multiple names |

## ACME / Let's Encrypt

For public certificates, Capper integrates ACME issuance and renews automatically
on a schedule. Bind a certificate to the service that should present it (load
balancer, ingress, or the API itself).

## Using a certificate with the API

Point the API at an issued cert/key to serve HTTPS directly:

```bash
capper api start --tls-cert /path/server.crt --tls-key /path/server.key
```

See [Configuration](../getting-started/configuration.md) and the
[Security model](../concepts/security-model.md).

## Renewal

The renewal scheduler reissues certificates before expiry and updates bindings.
Monitor upcoming expiries with `capper cert list` and the
[observability](observability.md) alerts.

## Related

- [Security model](../concepts/security-model.md) · [Ingress](ingress.md)
  · [Load balancers](load-balancers.md)
