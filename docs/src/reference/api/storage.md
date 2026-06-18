---
title: "API reference — storage"
description: "Endpoints for volumes, buckets, CSD volumes, and the S3 server."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# API reference — storage

All control-plane paths are under `/api/v1` and require
[authentication](overview.md).

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/storage/volumes` | list block volumes |
| `DELETE` | `/storage/volumes/{name}` | delete a volume |
| `GET` | `/storage/buckets` | list buckets |
| `DELETE` | `/storage/buckets/{bucket}` | delete a bucket |
| `GET`/`PUT`/`DELETE` | `/s3/buckets/{bucket}/policy` | bucket policy |
| `GET` | `/csd/volumes` | list CSD shared volumes |
| `GET` | `/csd/volumes/{vol}` | inspect a CSD volume |
| `GET` | `/csd/volumes/{vol}/{attachments,leases,replicas,snapshots}` | CSD volume detail |

## S3 data plane

The S3-compatible object API is served separately (standard S3 verbs on
`/{bucket}` and `/{bucket}/{key}`), authenticated with S3 credentials and bucket
policies — point any S3 client at it. Create credentials with
`capper storage bucket credentials`.

The [Go SDK](../sdk/go.md) `Storage` and `CSD` groups wrap the control-plane
endpoints.

## Related

- [API overview](overview.md) · [Manage storage](../../operator-guide/manage-storage.md)
  · [Storage model](../../concepts/storage-model.md)
