---
title: "Manage storage"
description: "Block volumes, the S3-compatible object store, snapshots, and shared CSD volumes."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Manage storage

`capper storage` manages four storage shapes: `volume`, `bucket`/`object`/`s3`,
`snapshot`, and `share` (CSD). For the conceptual model see the
[Storage model](../concepts/storage-model.md).

## Block volumes

```bash
capper storage volume create my-vol --size 20G --class local --encrypted
capper storage volume list
capper storage volume inspect my-vol
capper storage volume attach my-vol --instance <id>
capper storage volume detach my-vol --instance <id>
capper storage volume delete my-vol
```

| Flag (`volume create`) | Purpose |
| --- | --- |
| `--size <size>` | size hint, e.g. `20G` |
| `--class <class>` | volume class (default `local`) |
| `--encrypted` | mark the volume encrypted |

Detach before deleting; snapshot before destructive operations.

## Snapshots

```bash
capper storage snapshot create my-vol
capper storage snapshot list
# restore a snapshot to a new volume, then attach it
```

## Object store (S3-compatible)

```bash
capper storage bucket create my-bucket
capper storage bucket credentials create my-bucket   # S3 access/secret keys
capper storage object put my-bucket ./file.txt
capper storage object list my-bucket
capper storage s3 ...                                 # S3 server admin
```

Object access is controlled by S3 credentials and per-bucket policies. The server
handles keys path-traversal-safely. Point any S3 client at the endpoint with the
generated credentials.

## Shared/replicated volumes (CSD)

`capper storage share` manages CSD volumes that multiple nodes can mount (realized
on the node via FUSE) for clustered workloads.

## Backups

Protect data with [backup policies](manage-backups.md). For the control-plane
**database**, back it up separately — see the
[CapDB backup section](capdb-backend.md#operations).

## Related

- [Storage model](../concepts/storage-model.md) · [Manage backups](manage-backups.md)
  · [Manage instances](manage-instances.md)
