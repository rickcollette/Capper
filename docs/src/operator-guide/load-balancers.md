---
title: "Load balancers"
description: "Distribute traffic across instance backends, with listeners and public IPs."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Load balancers

`capper lb` creates load balancers that distribute traffic across instance
backends. A load balancer's listen address can come from an allocated
[public IP](routable-ips.md).

## Create and manage

```bash
capper lb create my-lb \
  --listen 0.0.0.0:8080 \
  --mode http \
  --algo round-robin \
  --select tier=web \
  --network app-net
capper lb backend ...           # add/remove instance backends
capper lb list
capper lb inspect my-lb
capper lb logs my-lb
capper lb publish my-lb         # publish/expose the listener
capper lb delete my-lb
```

| Flag (`lb create`) | Purpose |
| --- | --- |
| `--listen <addr>` | listen address, e.g. `0.0.0.0:8080` |
| `--mode tcp\|http` | proxy mode (default `tcp`) |
| `--algo round-robin\|least-connections` | balancing algorithm |
| `--select KEY=VALUE` | service selector label for backends |
| `--network <name\|id>` | attach to a virtual network |
| `--tls-cert <name>` | TLS cert from the [cert store](certificates.md) |

## Backends and health

Select backends by label (`--select`) or manage them explicitly with
`capper lb backend`. Unhealthy backends are removed from rotation — integrate with
[DNS health checks](manage-dns.md) and [observability](observability.md). For HTTPS,
attach a [certificate](certificates.md) with `--tls-cert`.

## Exposure

- Front a load balancer with [DNS](manage-dns.md) names.
- Bind a [public/elastic IP](routable-ips.md) for external reachability.
- Terminate TLS with an issued [certificate](certificates.md).

## Related

- [Networking model](../concepts/networking-model.md) · [Ingress](ingress.md)
  · [Public IPAM](routable-ips.md)
