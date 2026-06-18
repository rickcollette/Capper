---
title: "Database Administrator"
description: "Guide for DBAs managing Capper-hosted databases."
owner: "docs"
status: "stable"
reviewed: "2026-06-10"
outputs:
  - markdown
  - web
  - pdf
---

# Database Administrator

Capper hosts Postgres, Redis, and MariaDB instances with automated backup,
point-in-time restore, and read replica support.

## Databases

The Databases page lists all managed database instances with their engine,
version, status, and endpoint. Create a new instance by specifying engine,
storage size, and the VPC it should join.

![Databases](/assets/images/screenshots/15-databases.png)

## Storage — Backing Volumes

Each database instance is backed by a Capper block volume. The Storage page
lets you inspect volume utilisation and expand capacity without downtime.

![Storage](/assets/images/screenshots/05-storage.png)

## Backups and Restore

The Backups page shows manual and scheduled snapshots. Define a backup policy
with an interval (hourly, daily, weekly) and a retention count. To restore,
select a snapshot and click **Restore** — Capper creates a new database
instance from the snapshot and updates its DNS record.

![Backups](/assets/images/screenshots/16-backups.png)

## Key Workflows

- [Create a Database Backup Policy](../operator-guide/manage-backups.md)
- [Restore a Database from Snapshot](../operator-guide/manage-backups.md)
