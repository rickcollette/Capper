# Capper

Capper is a local experimental capsule image runner for `.cap` images.

Do not run untrusted `.cap` images with Capper v0.

## Bootstrap Alpine

```bash
sh examples/alpine/bootstrap.sh
go run ./cmd/capper --store /tmp/capper-alpine create alpine.cap examples/alpine/capper.json
```

## Run In Userland

Capper prefers Bubblewrap (`bwrap`) when it is installed and unprivileged user namespaces are available:

```bash
go run ./cmd/capper --store /tmp/capper-alpine run alpine.cap
go run ./cmd/capper --store /tmp/capper-alpine list instances
go run ./cmd/capper --store /tmp/capper-alpine stop INSTANCE_ID
```

If `bwrap` is unavailable, Capper falls back to chroot and may require `sudo`.

You can choose the runtime explicitly:

```bash
go run ./cmd/capper --runtime bwrap --store /tmp/capper-alpine run alpine.cap
sudo go run ./cmd/capper --runtime chroot --store /tmp/capper-alpine run alpine.cap
```

Limit resources for a run:

```bash
go run ./cmd/capper --store /tmp/capper-alpine run --memory 128M --cpu-time 60 --file-size 16M alpine.cap
```

`--pids` is recorded for Bubblewrap runs and enforced for chroot runs in v0. Full CPU quotas and pids enforcement for rootless Bubblewrap need cgroup support.

## Build And Test

```bash
go test ./...
go build -o /tmp/capper ./cmd/capper
```

## One-Command Local Run

Build a fresh runnable bundle, copy it into `capper-run/`, and start the API
plus control-plane daemon:

```bash
make capper-run
```

The service listens at `http://127.0.0.1:8687` by default. Logs and the PID file
are written under `capper-run/logs/` and `capper-run/run/`.

Useful follow-ups:

```bash
make capper-run-status
make capper-run-stop
```

Override defaults when needed:

```bash
CAPPER_RUN_API_ADDR=127.0.0.1:8690 make capper-run
CAPPER_RUN_CONSOLE=/path/to/CapperWeb/dist make capper-run
```
