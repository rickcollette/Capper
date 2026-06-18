# Alpine Example

Busybox-free Alpine Linux rootfs for Capper capsules. Built via Docker: installs
**bash**, **coreutils**, **procps-ng**, **util-linux**, and related packages, then
**removes busybox entirely** from the exported rootfs.

Requires docker on the build host (same as `examples/alma`).

Bootstrap it:

```bash
sh examples/alpine/bootstrap.sh
```

Create a `.cap` image:

```bash
go run ./cmd/capper --store /tmp/capper-alpine create alpine.cap examples/alpine/capper.json
```

Run it with chroot privileges:

```bash
sudo go run ./cmd/capper --store /tmp/capper-alpine run alpine.cap
```

Connect:

```bash
sudo go run ./cmd/capper --store /tmp/capper-alpine connect INSTANCE_NAME_OR_ID
```

Verify no busybox inside a running instance:

```bash
find / -name '*busybox*'  # should return nothing
command -v ls free ps      # GNU/procps binaries
```
