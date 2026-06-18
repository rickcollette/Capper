---
title: "Observability: inventory, drift, metrics, and alerts."
description: "Operate the Capper Resource Monitor (capper-observe)."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Observability

The Capper Resource Monitor (`capper-observe`) gives a single, searchable view
of every resource: a unified inventory, configuration history with drift
detection, time-series metrics, resource events, and alerts.

## Unified inventory

`capper resources sync` projects live resources (instances, networks, nodes,
load balancers, …) into one inventory and reconciles vanished ones.

```bash
capper resources sync
capper resources list --type instance
capper resources get <resource-id>
```

API: `GET /api/v1/resources`, `GET /api/v1/resources/{id}`. SDK: `c.Resources`.

## Metrics

The `capper-agent` pushes host metrics (CPU, memory, disk, load) from each node
every heartbeat. You can also ingest custom samples and query any series.

```bash
capper metrics ingest --resource-type node --resource-id <id> --metric cpu.percent --value 42
capper metrics query  --resource-type node --resource-id <id> --metric cpu.percent --range 1h
```

API: `POST /api/v1/metrics/ingest`, `GET /api/v1/metrics/query`. Per-resource
monitoring summaries are at
`GET /api/v1/{instances,nodes,networks,load-balancers,certificates}/{id}/monitoring`.

## Config drift

Record a resource's desired configuration, report the observed configuration,
and Capper classifies drift (`in_sync`, `drifted`, `unknown`). Repair resets the
baseline to the desired config.

```bash
capper config drift list
capper config drift repair <resource-id>
```

## Alerts

Alert rules evaluate the latest metric sample against a threshold and open
alerts (deduplicated per rule + resource). Alerts move open → acknowledged →
resolved.

```bash
capper alerts rules         # list rules
capper alerts list          # open alerts
```

API: `GET /api/v1/alerts`, `POST /api/v1/alerts/rules`,
`POST /api/v1/alerts/{id}/ack`, `POST /api/v1/alerts/{id}/resolve`.

In the Web UI: **Observability → Resources** and **Observability → Alerts**, plus
a **Monitoring** tab on node detail pages.
