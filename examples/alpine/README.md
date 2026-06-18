# Alpine Example

This example uses Alpine Linux minirootfs as a tiny real root filesystem for Capper.

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
