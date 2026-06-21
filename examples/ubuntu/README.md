# Ubuntu 24.04 Capsule

A minimal Ubuntu 24.04 LTS rootfs optimized for Capper containers.

## Build

Requires Docker on the build host.

```bash
cd examples/ubuntu
bash bootstrap.sh
```

This will:
1. Pull the official Ubuntu 24.04 image
2. Install core tools (bash, coreutils, procps, util-linux, iproute2, etc.)
3. Remove unnecessary packages and cache
4. Export the rootfs to `rootfs/`

Size: ~200 MB

## Usage

Once built, the rootfs is ready for packaging into a `.cap` capsule file:

```bash
# Create a capsule from the rootfs
capper capsule create ubuntu ./examples/ubuntu/capper.json -o ubuntu.cap
```

Or run directly:

```bash
capper run -i examples/ubuntu ./examples/ubuntu/capper.json
```

## Environment

- **Base Image**: ubuntu:24.04
- **Shell**: bash
- **Package Manager**: apt-get
- **Memory**: 256 MB default
- **CPU**: 300 seconds limit
- **Processes**: 256 max

## Customize

Edit `capper.json` to adjust resource limits, environment variables, or the entrypoint.

Edit `bootstrap.sh` to add additional packages during build (e.g., `apt-get install python3`).
