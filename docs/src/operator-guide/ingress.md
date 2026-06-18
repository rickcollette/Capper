---
title: "Ingress"
description: "Host/path routing rules in front of services."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Ingress

`capper ingress` manages host/path routing rules that sit in front of services,
routing incoming requests to the right backend.

## Manage rules

```bash
capper ingress create --host app.example.com --path / --backend web-lb \
  --tls-cert app-cert --rate-limit 600
capper ingress list
capper ingress delete <rule-id>
```

| Flag (`ingress create`) | Purpose |
| --- | --- |
| `--host <name>` | hostname to match |
| `--path <prefix>` | path prefix to match (default `/`) |
| `--backend <lb>` | backend load balancer name |
| `--tls-cert <name>` | TLS cert from the [cert store](certificates.md) |
| `--rate-limit <rpm>` | requests per minute (0 = unlimited) |

## How it fits

Ingress complements [load balancers](load-balancers.md): a load balancer spreads
traffic across instances, while ingress chooses *which* service a request reaches
based on host and path. Pair with [DNS](manage-dns.md) for names,
[certificates](certificates.md) for TLS, and [public IPs](routable-ips.md) for
external reachability.

## Related

- [Networking model](../concepts/networking-model.md) · [Load balancers](load-balancers.md)
  · [Manage DNS](manage-dns.md) · [Certificates](certificates.md)
