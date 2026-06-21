#!/bin/sh
# Build an Ubuntu rootfs for Capper capsules.
# Uses official Ubuntu container image with full core OS toolset.
# Requires docker on the build host.
set -eu

cd "$(dirname "$0")"
img="${UBUNTU_IMAGE:-ubuntu:24.04}"

if ! command -v docker >/dev/null 2>&1; then
  echo "ubuntu bootstrap: docker is required to build the Ubuntu rootfs" >&2
  exit 1
fi

docker pull -q "$img" >/dev/null
name="capper-ubuntu-build-$$"
docker rm -f "$name" >/dev/null 2>&1 || true

docker run --name "$name" "$img" bash -c \
  "apt-get update && apt-get install -y --no-install-recommends bash coreutils procps util-linux iproute2 iputils-ping findutils which tar gzip less gawk sed grep ca-certificates && apt-get clean && rm -rf /var/cache/apt /var/lib/apt/lists/* /tmp/* /var/log/*" >/dev/null

rm -rf rootfs && mkdir rootfs
docker export "$name" | tar -x -C rootfs 2>/dev/null
docker rm -f "$name" >/dev/null

chmod -R u+rwX rootfs

ln -sf bash rootfs/bin/sh
mkdir -p rootfs/etc/profile.d
cat > rootfs/etc/profile.d/capper-tools.sh <<'EOF'
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EOF
cat > rootfs/etc/profile.d/capper-prompt.sh <<'EOF'
# Capper login prompt: user@hostname:cwd#
export PS1='\u@\h:\w# '
EOF

# Update bashrc for consistent prompt in non-login shells
if ! grep -q 'Capper login prompt' rootfs/etc/bash.bashrc 2>/dev/null; then
  cat >> rootfs/etc/bash.bashrc <<'EOF'

# Capper login prompt: user@hostname:cwd#
if [ -n "${PS1-}" ]; then
  PS1='\u@\h:\w# '
fi
EOF
fi

if find rootfs -name "*busybox*" 2>/dev/null | grep -q .; then
  echo "ubuntu bootstrap: unexpected busybox in rootfs:" >&2
  find rootfs -name "*busybox*" 2>/dev/null >&2
  exit 1
fi

echo "Ubuntu rootfs ready at examples/ubuntu/rootfs ($(du -sh rootfs | cut -f1))"
