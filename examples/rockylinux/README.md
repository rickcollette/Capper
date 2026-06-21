# Rocky Linux 9 Capsule

A minimal Rocky Linux 9 rootfs optimized for Capper containers.

## Build

Requires Docker on the build host.

```bash
cd examples/rockylinux
bash bootstrap.sh
```

This will:
1. Pull the official Rocky Linux 9 image
2. Install core tools (bash, coreutils, procps-ng, util-linux, iproute, etc.)
3. Remove unnecessary packages and cache
4. Export the rootfs to `rootfs/`

Size: ~200 MB

## Usage

Once built, the rootfs is ready for packaging into a `.cap` capsule file:

```bash
# Create a capsule from the rootfs
capper capsule create rockylinux ./examples/rockylinux/capper.json -o rockylinux.cap
```

Or run directly:

```bash
capper run -i examples/rockylinux ./examples/rockylinux/capper.json
```

## Environment

- **Base Image**: rockylinux:9
- **Shell**: bash
- **Package Manager**: dnf
- **Memory**: 256 MB default
- **CPU**: 300 seconds limit
- **Processes**: 256 max

## Requirements

Rocky Linux 9 requires x86-64-v2 CPU features (SSE4.2, etc.). It will fail on older hosts with:
```
Fatal glibc error: CPU does not support x86-64-v2
```

For older hardware, use Rocky Linux 8 instead:
```bash
ROCKYLINUX_IMAGE=rockylinux:8 bash bootstrap.sh
```

## Customize

Edit `capper.json` to adjust resource limits, environment variables, or the entrypoint.

Edit `bootstrap.sh` to add additional packages during build (e.g., `dnf -y install python3`).
