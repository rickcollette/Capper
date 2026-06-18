# Capper Quickstart

Capper is a local experimental `.cap` image runner.

Do not run untrusted `.cap` images with Capper v0.

## Build

From the repo root:

```bash
make build
```

The binary is written to:

```text
bin/capper
```

Run tests:

```bash
make test
```

Clean generated build output:

```bash
make clean
```

## Start A Local Capper Service

For local development, this is the simplest path:

```bash
make capper-run
```

That target rebuilds `DIST/`, copies the runnable bundle to `capper-run/`, stops
any previous service from that folder, starts the API with the daemon embedded,
and waits for `/api/v1/health` to pass.

Defaults:

```text
URL:  http://127.0.0.1:8687
PID:  capper-run/run/api.pid
Log:  capper-run/logs/api.log
Store: capper-run/store
```

Check or stop it:

```bash
make capper-run-status
make capper-run-stop
```

If the default port is busy:

```bash
CAPPER_RUN_API_ADDR=127.0.0.1:8690 make capper-run
```

## Create A Runnable Distribution

Build a self-contained distribution folder:

```bash
make dist
```

This creates:

```text
DIST/
├── capper
├── lib/
│   └── capper-bin
├── alpine.cap
├── store/
│   ├── capper.db
│   └── images/
│       └── alpine.cap
├── docs/
├── examples/
├── schemas/
└── README.md
```

Use Capper from inside `DIST/`:

```bash
cd DIST
./capper --json list images
```

## Runtime Modes

Capper supports three runtime modes:

```bash
--runtime auto
--runtime bwrap
--runtime chroot
```

`auto` is the default. It uses Bubblewrap when available, then falls back to chroot.

Use rootless Bubblewrap:

```bash
./capper --runtime bwrap run alpine.cap
```

Use chroot with sudo:

```bash
sudo ./capper --runtime chroot run alpine.cap
```

Plain chroot requires root privileges or `CAP_SYS_CHROOT`.

## Run The Included Alpine Capfile

After `make dist`:

```bash
cd DIST
./capper run alpine.cap
```

Expected output:

```text
Started instance

Name:   alpine-...
ID:     ...
Image:  alpine.cap
PID:    ...
Status: running
```

List instances:

```bash
./capper list instances
```

JSON output:

```bash
./capper --json list instances
```

The Alpine image writes logs under the instance directory:

```bash
cat store/instances/INSTANCE_ID/stdout.log
cat store/instances/INSTANCE_ID/stderr.log
```

Expected stdout:

```text
hello from alpine
3.23.0
```

Stop the instance:

```bash
./capper stop INSTANCE_ID
```

You can use either the instance ID or generated name:

```bash
./capper stop alpine-steady-raven
```

## Connect To A Running Instance

Start an instance:

```bash
./capper run alpine.cap
```

Connect to a shell:

```bash
./capper connect INSTANCE_ID
```

Capper tries these shells in order:

```text
configured shell
/bin/sh
/bin/bash
/busybox/sh
```

The shell runs as the capsule's configured `user.uid` and `user.gid`.

Exit the shell:

```bash
exit
```

Stop the instance when finished:

```bash
./capper stop INSTANCE_ID
```

## Create Your Own Capfile

A `.cap` file is a tar archive containing:

```text
capsule.json
rootfs.tar.zst
checksums.json
```

You do not write these files manually for normal use. Create a config JSON file and let Capper package it.

Example project:

```text
myapp/
├── capper.json
└── rootfs/
    ├── bin/
    │   └── sh
    └── app/
```

Example `capper.json`:

```json
{
  "name": "myapp",
  "version": "0.1.0",
  "rootfs": "./rootfs",
  "entrypoint": ["/bin/sh"],
  "args": ["-c", "echo hello from myapp && sleep 3600"],
  "env": {
    "PATH": "/sbin:/bin:/usr/sbin:/usr/bin"
  },
  "workingDir": "/",
  "shell": "/bin/sh",
  "user": {
    "uid": 0,
    "gid": 0
  },
  "network": {
    "enabled": false
  },
  "resources": {
    "memoryBytes": 134217728,
    "cpuTimeSecs": 60,
    "maxProcesses": 64,
    "fileSizeBytes": 16777216
  }
}
```

Required fields:

```text
name
version
rootfs
entrypoint
```

Create the capfile:

```bash
capper --store ./store create myapp.cap myapp/capper.json
```

Run it:

```bash
capper --store ./store run myapp.cap
```

## Limit Capsule Resources

Resource limits belong in `capper.json` when they should travel with the `.cap` image. Capper copies them into `capsule.json` at create time and records the effective values in `instance.json` at run time.

You can also override them for one run:

```bash
capper run --memory 128M --cpu-time 60 --pids 64 --file-size 16M alpine.cap
```

The `resources` block uses bytes and seconds:

```json
{
  "resources": {
    "memoryBytes": 134217728,
    "cpuTimeSecs": 60,
    "maxProcesses": 64,
    "fileSizeBytes": 16777216
  }
}
```

Current v0 limits:

```text
--memory     virtual memory/address-space limit
--cpu-time   CPU time limit in seconds
--file-size  maximum file size the capsule can write
--pids       recorded for bwrap, enforced only by chroot in v0
```

Precise CPU shares/quotas and bwrap process-count enforcement require cgroups and are planned after v0 rlimit support.

## Bootstrap A Fresh Alpine Rootfs

The repo includes an Alpine example:

```bash
sh examples/alpine/bootstrap.sh
```

This downloads Alpine minirootfs, verifies its SHA-512 checksum, and extracts:

```text
examples/alpine/rootfs/
```

Create an Alpine capfile:

```bash
bin/capper --store /tmp/capper-alpine create alpine.cap examples/alpine/capper.json
```

Run it:

```bash
bin/capper --store /tmp/capper-alpine run alpine.cap
```

## Image Commands

List images:

```bash
capper --store ./store list images
```

List images as JSON:

```bash
capper --store ./store --json list images
```

Delete an image:

```bash
capper --store ./store delete alpine.cap
```

Capper refuses to delete an image while running instances still use it.

## Instance Commands

Run:

```bash
capper --store ./store run alpine.cap
```

List:

```bash
capper --store ./store list instances
```

Connect:

```bash
capper --store ./store connect INSTANCE_ID
```

Stop:

```bash
capper --store ./store stop INSTANCE_ID
```

Stop immediately with `SIGKILL`:

```bash
capper --store ./store stop --kill INSTANCE_ID
```

Use a custom stop timeout:

```bash
capper --store ./store stop --timeout 10 INSTANCE_ID
```

## Store Layout

Capper stores local state under `--store`.

Example:

```text
store/
├── capper.db
├── images/
│   └── alpine.cap
├── instances/
│   └── INSTANCE_ID/
│       ├── instance.json
│       ├── pid
│       ├── rootfs/
│       ├── stdout.log
│       └── stderr.log
└── tmp/
```
