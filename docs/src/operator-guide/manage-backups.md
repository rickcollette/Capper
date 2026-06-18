---
title: "Manage backups"
description: "On-demand backups, scheduled retention policies, and restore."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Manage backups

`capper backup` covers on-demand backups, scheduled **backup policies** with
retention, and restore.

## On-demand

```bash
capper backup create --resource <id>
capper backup list
capper backup restore <backup-id>
```

## Policies (scheduled + retained)

```bash
capper backup policy-create --schedule '@daily' --retain 7 --resource <id>
capper backup policy-list
capper backup policy-delete <policy-id>
```

A policy runs on a schedule and prunes to the retention count. Use
`capper backup test` to validate a policy/configuration.

## What to back up

- **Workload data** — volumes (snapshot + backup), buckets, CSD shares. See
  [Manage storage](manage-storage.md).
- **Control-plane database** — backed up separately. With the embedded backend,
  snapshot `~/.capper/capper.db` with the daemon stopped; with the networked
  backend, use the online `.backup` flow in the
  [CapDB operations section](capdb-backend.md#operations).

## Restore drills

Test restores periodically — an untested backup is a hope, not a recovery plan.
Restore to a new resource first and verify before cutting over.

## Related

- [Manage storage](manage-storage.md) · [Storage model](../concepts/storage-model.md)
  · [CapDB backend](capdb-backend.md)
