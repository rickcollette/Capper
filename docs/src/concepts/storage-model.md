---
title: "Storage model"
description: "Block volumes, the S3-compatible object store, snapshots, CSD, and backups."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Storage model

Capper offers four storage shapes plus a backup layer, all managed through
`capper storage` (and `capper backup`):

- **Block volumes** — persistent disks attached to instances. Create, attach,
  detach, resize, snapshot.
- **Object store (S3-compatible)** — buckets and objects served by a built-in
  S3 server with bucket policies and S3 credentials. Use any S3 client.
- **Snapshots** — point-in-time copies of volumes; restore to a new volume.
- **CSD (shared/replicated volumes)** — volumes that can be mounted across nodes,
  backed by a CSD server the control plane manages.

## Block volumes

```bash
capper storage volume create --size 20G my-vol
capper storage volume attach my-vol --instance <id>
capper storage snapshot create my-vol
```

Volumes live in a project and are placed onto nodes by topology. Detach before
deleting; snapshot before destructive changes.

## Object store

The S3 server exposes standard bucket/object operations with path-traversal-safe
key handling and per-bucket policies:

```bash
capper storage bucket create my-bucket
capper storage object put my-bucket ./file.txt
capper storage object list my-bucket
```

Access control is via S3 credentials and bucket policies; see
[Manage storage](../operator-guide/manage-storage.md).

## CSD shared volumes

CSD provides shared/replicated volumes that multiple nodes can mount (e.g. for
clustered workloads). The control plane runs a CSD server and tracks attachments;
mounts are realized on the node via FUSE.

## Backups

`capper backup` manages backups and **backup policies** (scheduled, retained):

```bash
capper backup create --resource <id>
capper backup policy create --schedule '@daily' --retain 7
capper backup list
```

Backups cover platform resources; for the control-plane **database** itself, see
the [CapDB backup section](../operator-guide/capdb-backend.md#operations) (online
`.backup`) or snapshot the embedded SQLite file with the daemon stopped.

## Control-plane state vs workload storage

Do not confuse the two: **workload storage** (volumes, buckets, CSD) is what your
instances use; **control-plane state** is the single database that records all
platform metadata (see [Architecture](architecture.md) and the
[CapDB backend](../operator-guide/capdb-backend.md)). They are backed up
separately.

## Related

- [Manage storage](../operator-guide/manage-storage.md)
  · [Manage backups](../operator-guide/manage-backups.md)
  · [CapDB backend](../operator-guide/capdb-backend.md)
