---
title: "Runbooks"
description: "Operational procedures for common incidents and maintenance."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Runbooks

Step-by-step procedures for common operational tasks and incidents. Each assumes a
running control plane; start every investigation with `capper status`,
`capper aio status`, and `capper aio logs`.

## Node maintenance

1. `capper node cordon <node>` — stop new placements.
2. `capper node drain <node>` — move workloads off.
3. Perform maintenance.
4. `capper node uncordon <node>` (or re-approve) — return it to service.

For wider scope, use `capper zone drain` / `capper region evacuate`. See
[Topology & nodes](../operator-guide/topology.md).

## Control-plane database restart (CapDB)

A `capdb-server` restart is designed to be a blip: pooled connections self-heal and
the next query reconnects within seconds. If queries keep failing:

1. Check `capdb-server` is listening and healthy.
2. Verify the DSN, token file, and (for TLS) that `ca=` matches the server cert
   SAN.
3. Confirm startup retries / `connect_timeout` are configured. See
   [CapDB backend](../operator-guide/capdb-backend.md) and
   [Troubleshooting](../getting-started/troubleshooting.md).

## Backup and restore drill

1. Take a backup (`capper backup create`, or CapDB online `.backup`).
2. Restore to a **new** resource and verify before cutting over.
3. Record RPO/RTO. See [Manage backups](../operator-guide/manage-backups.md).

## Rotate a leaked token or key

1. Revoke the token: `capper iam token` (revocation takes effect within seconds).
2. Rotate KMS keys (`capper kms key rotate`) and re-issue secrets if a master key
   is suspected compromised.
3. Audit access: `capper iam audit`.

## Certificate expiry

The renewal scheduler reissues before expiry; if a cert lapses, reissue with
`capper cert issue` and rebind. Monitor upcoming expiries via
[observability](../operator-guide/observability.md) alerts.

## Suspended account / access denied

- `403 account suspended` → reactivate the account (`capper org` account commands).
- `403 not authorized for account/org/project` → the principal lacks membership for
  the requested tenant scope; fix membership or use an authorized principal. See the
  [Security model](../concepts/security-model.md).

## Related

- [Operator guide](../operator-guide/index.md) · [Troubleshooting](../getting-started/troubleshooting.md)
