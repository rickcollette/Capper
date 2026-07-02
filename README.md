# Capper

Capper is a free and open source, self-hosted cloud control plane for running
private infrastructure on Linux. It brings together a CLI, REST API, Web console,
node agent, capsule runtime, networking, storage, identity, topology, and
all-in-one node operations in one project.

Current release line: **0.1.38 beta**. Capper is usable for evaluation,
development, demos, and early operator feedback, but it is still pre-1.0
software. Do not treat capsule isolation as a strong security boundary for
hostile workloads yet.

Repository: <https://github.com/rickcollette/Capper>

## What is included

The current tree includes:

| Area | Current implementation |
| --- | --- |
| CLI | `capper` command tree generated from source, including AIO, API, compute, networking, storage, IAM, topology, registry, backups, jobs, events, MCP, AI, and admin commands. |
| API | REST API under `/api/v1`, served by `capper api start`, with bearer-token auth and optional embedded Web console. |
| Web console | CapperWeb static assets can be served by the API with `--console <dist>` and are bundled into AIO release archives. |
| Node services | `capper-agent` for node heartbeat, inventory, metrics, and service supervision; `capinit` for capsule PID 1. |
| Runtime | Local `.cap` capsule creation and execution with `bwrap`, `chroot`, `crun`, or `runc`. |
| AIO install | Guided `install.sh` in each release archive installs host dependencies, Docker Engine, Docker Compose plugin, binaries, console assets, systemd drop-ins, and versioned symlinks. |
| Database | Embedded pure-Go SQLite by default; optional CapDB backend for networked, pooled control-plane storage. |
| Release packaging | Per-distro x86_64 `.tgz` bundles built inside matching Docker images so glibc/OpenSSL expectations are explicit. |

## Install a beta AIO bundle

The recommended beta path is to install one of the prebuilt AIO tarballs from
GitHub Releases. Pick the artifact that matches the target operating system and
glibc family.

Latest beta release:
<https://github.com/rickcollette/Capper/releases/tag/v0.1.38-beta.2>

| Target | Artifact |
| --- | --- |
| Ubuntu 24.04 x86_64 | `capper-aio-0.1.38-ubuntu24.04-glibc2.39-x86_64.tgz` |
| Debian 12 x86_64 | `capper-aio-0.1.38-debian12-glibc2.36-x86_64.tgz` |
| RHEL 9 x86_64 | `capper-aio-0.1.38-rhel9-glibc2.34-x86_64.tgz` |
| Rocky Linux current x86_64 | `capper-aio-0.1.38-rocky10-glibc-detect-x86_64.tgz` |
| Ubuntu 18.04 x86_64 | `capper-aio-0.1.38-ubuntu18.04-glibc2.27-x86_64.tgz` |

Example for Ubuntu 24.04:

```bash
release=https://github.com/rickcollette/Capper/releases/download/v0.1.38-beta.2
bundle=capper-aio-0.1.38-ubuntu24.04-glibc2.39-x86_64.tgz

curl -LO "$release/$bundle"
curl -LO "$release/$bundle.sha256"
sha256sum -c "$bundle.sha256"

tar xzf "$bundle"
cd "${bundle%.tgz}"

sudo ./install.sh --check-only
sudo ./install.sh --yes
sudo capper aio doctor
sudo capper aio init --backend capdb
sudo capper aio up
capper aio status
```

The installer stages releases under `/usr/local/lib/capper/<version>`, points
`/usr/local/lib/capper/current` at the active version, links binaries into
`/usr/local/bin`, and links the bundled console into `/opt/capper/console`.
Re-running the installer stages the new version as an upgrade instead of
overwriting the old one.

Useful installer options:

```bash
sudo ./install.sh --help
sudo ./install.sh --check-only
sudo ./install.sh --doctor-only
sudo ./install.sh --yes --skip-docker
sudo ./install.sh --yes --backend sqlite
```

## Build from source

Prerequisites for the default build:

- Linux x86_64
- Go 1.25 or newer
- `bwrap`, `crun`, `runc`, or `chroot` support for capsule execution
- Node.js only if you are building CapperWeb yourself

Build and test the default pure-Go binaries:

```bash
make build
make test
```

This writes `bin/capper` and uses the embedded SQLite backend.

Build with the optional CapDB backend:

```bash
make capdb-fetch
make build-capdb
make test-capdb
```

`make capdb-fetch` checks out the CapDB engine into this repository's ignored
`./CapDB` directory. Do not build against a sibling or external CapDB working
copy unless you intentionally override `CAPDB_DIR`.

## Local development run

For a local control-plane service:

```bash
make capper-run
make capper-run-status
make capper-run-stop
```

Defaults:

```text
URL:   http://127.0.0.1:8687
PID:   capper-run/run/api.pid
Log:   capper-run/logs/api.log
Store: capper-run/store
```

Useful overrides:

```bash
CAPPER_RUN_API_ADDR=127.0.0.1:8690 make capper-run
CAPPER_RUN_CONSOLE=/path/to/CapperWeb/dist make capper-run
```

Start the API directly:

```bash
capper api start --listen 127.0.0.1:8686 --console /path/to/CapperWeb/dist
```

## Run a local capsule

The capsule runner is still available for local testing:

```bash
sh examples/alpine/bootstrap.sh
go run ./cmd/capper --store /tmp/capper-alpine create alpine.cap examples/alpine/capper.json
go run ./cmd/capper --store /tmp/capper-alpine run alpine.cap
go run ./cmd/capper --store /tmp/capper-alpine list instances
```

Capper selects a runtime automatically, or you can choose one:

```bash
go run ./cmd/capper --runtime bwrap --store /tmp/capper-alpine run alpine.cap
go run ./cmd/capper --runtime crun --store /tmp/capper-alpine run alpine.cap
sudo go run ./cmd/capper --runtime chroot --store /tmp/capper-alpine run alpine.cap
```

## Build release artifacts

Release outputs are written to `DIST/AIO/` and are intended to be uploaded to
GitHub Releases, not committed to git.

Build the full matrix:

```bash
rm -rf DIST/AIO/*
SKIP_TESTS=1 scripts/release-matrix.sh 0.1.38
```

The current matrix builds inside Docker for:

- Ubuntu 24.04, glibc 2.39
- Debian 12, glibc 2.36
- RHEL 9, glibc 2.34
- Rocky Linux current, detected glibc
- Ubuntu 18.04, glibc 2.27

Publish a beta prerelease with the GitHub CLI:

```bash
scripts/github-release-beta.sh 0.1.38 2
```

This creates or updates `v0.1.38-beta.2` and uploads every `capper-aio-*.tgz`,
matching `.sha256` file, and `channels.json`.

## Documentation

Source documentation lives under `docs/src/` and generated references are checked
into the docs tree.

High-signal starting points:

- [Install and build](docs/src/getting-started/installation.md)
- [Quickstart](docs/src/getting-started/quickstart.md)
- [Beta releases](docs/src/operator-guide/beta-releases.md)
- [CapDB backend](docs/src/operator-guide/capdb-backend.md)
- [CLI reference](docs/src/reference/cli/capper.md)
- [API route reference](docs/src/reference/api/routes.md)
- [Repository layout](docs/src/developer-guide/repository-layout.md)

Regenerate documentation references:

```bash
make docs-gen
make docs-check
```

## Contributing

Capper is an open source project and contributor help is welcome. The best way
to sign up is to open a GitHub issue describing what you want to work on, where
you want help getting oriented, or what beta feedback you can provide.

When changing API CRUD behavior, update CapperWeb in the matching workflow so
the frontend and backend stay aligned.
