# Capper

Capper is a self-hosted **multi-tenant cloud control plane**. It started as a
local `.cap` capsule image runner and has grown into a full platform: compute,
networking, storage, identity, topology, certificates, observability,
serverless, and public IP management — all driven by a single control plane and
reachable through a CLI, a REST API, a Go SDK, and a Web UI.

> Do not run untrusted `.cap` images with Capper v0.

## Subsystems

| Area | What it provides |
|---|---|
| **Compute** | `.cap` capsule instances (bwrap/chroot/crun/runc), images, templates, instance types, GPU inventory, compute groups + autoscale |
| **Networking** | virtual networks, VPCs + subnets, firewalls, load balancers, DNS, ingress, **Public IPAM / Elastic IPs** |
| **Storage** | block volumes, S3-compatible object store, snapshots, CSD shared/replicated volumes, backups |
| **Multi-tenancy** | organizations → accounts → projects, IAM (users/groups/roles/policies), managed policies, assume-role, quotas, governance, audit |
| **Topology** | realms → regions → zones → nodes, node pools, service roles, the `capper-agent` node daemon, placement scheduler |
| **VPC Mobility** | plan → approve → execute → cutover migration of VPC workloads across realms/regions |
| **Certificates** | ACME / Let's Encrypt issuance, renewal scheduler, bindings, internal CA |
| **Observability** | unified resource inventory, config drift, metrics, resource events, alerts (`capper-observe`) |
| **Serverless** | Lambda-style **Functions** (triggers, invocations) and managed **MCP servers** with per-tool IAM + approval gates |
| **Security** | KMS, secrets, image posture scanning, SBOM, marketplace review |

Every subsystem is exposed consistently across all four interfaces (CLI, REST
API, Go SDK, Web UI) and is covered by tests.

## Interfaces

- **CLI** — `capper <subsystem> <verb>` (e.g. `capper instances list`, `capper org create`, `capper fn invoke`, `capper ip-pool create`). Run `capper --help`.
- **REST API** — `capper api start` serves `/api/v1/...` with bearer-token auth.
- **Go SDK** — `import cappersdk "capper/sdk/go"`; `c := cappersdk.New(url, token)`. Groups include `c.Instances`, `c.IAM`, `c.Functions`, `c.IPAM`, `c.Resources`, and ~40 more.
- **Web UI** — [CapperWeb](../CapperWeb) (Vite + React), served via `capper api start --console <dist>`.

## Quick start (capsule)

```bash
sh examples/alpine/bootstrap.sh
go run ./cmd/capper --store /tmp/capper-alpine create alpine.cap examples/alpine/capper.json
go run ./cmd/capper --store /tmp/capper-alpine run alpine.cap
go run ./cmd/capper --store /tmp/capper-alpine list instances
```

Capper prefers Bubblewrap (`bwrap`) with unprivileged user namespaces; it falls
back to chroot (which may require `sudo`). Choose explicitly with
`--runtime bwrap|chroot|crun|runc`. Limit resources:

```bash
go run ./cmd/capper --store /tmp/capper-alpine run --memory 128M --cpu-time 60 --file-size 16M alpine.cap
```

## Run the control plane

```bash
# API + control-plane daemon, building a fresh bundle into capper-run/
make capper-run            # serves http://127.0.0.1:8687
make capper-run-status
make capper-run-stop
```

Override defaults:

```bash
CAPPER_RUN_API_ADDR=127.0.0.1:8690 make capper-run
CAPPER_RUN_CONSOLE=/path/to/CapperWeb/dist make capper-run   # serve the Web UI
```

Or start the API directly:

```bash
capper api start --listen 127.0.0.1:8686 --console /path/to/CapperWeb/dist
```

## All-in-one dev node

```bash
capper aio init      # create storage layout + local topology
capper aio up        # start API, daemon, and local services
capper aio status
```

## Node agent

A second machine joins the control plane as a worker node:

```bash
capper node join my-node --token <join-token> --address 10.0.0.5 --role compute
# then, on the control plane:
capper node approve my-node
```

The `capper-agent` daemon (`cmd/capper-agent`) runs on nodes: it sends
heartbeats, reports inventory, pushes host metrics, and supervises services.

## Build and test

```bash
go build ./...
go test ./...
go vet ./...
# Web UI
cd ../CapperWeb && npm run build
```

## Storage backend

By default Capper persists control-plane state in a single embedded SQLite
database (`modernc.org/sqlite`) opened with WAL + a busy timeout for concurrent
writers — pure-Go, no external process.

For networked, connection-pooled storage it can instead talk to **CapDB** (a
SQLite fork with a TLS client/server protocol and a native pool, maintained at
<https://github.com/rickcollette/CapDB> and consumed via `CAPDB_DIR`). This
backend is opt-in: build with `-tags capdb` and set
`CAPPER_DB_DRIVER=capdb` + `CAPPER_DB_DSN=capdb://…`. It keeps the SQLite
dialect, so no SQL changes are needed. See
[`docs/src/operator-guide/capdb-backend.md`](docs/src/operator-guide/capdb-backend.md).

```bash
make capdb && go build -tags capdb ./cmd/capper   # build the networked driver
make test-capdb                                    # driver conformance suite
```

## Documentation

Operator and concept docs live under [`docs/`](docs/) (built with the toolchain
in `docs/config.yml` + `docs/nav.yml`).
