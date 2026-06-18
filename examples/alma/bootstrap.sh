#!/bin/sh
# Build an AlmaLinux rootfs for the sample image by exporting the official
# container image with a full core OS toolset (no busybox — Alma never ships it).
# Default almalinux:8 — baseline x86-64. Alma 9+ requires x86-64-v2 (SSE4.2, etc.)
# and fails on older hosts with "Fatal glibc error: CPU does not support x86-64-v2".
# Requires docker on the build host.
set -eu

cd "$(dirname "$0")"
img="${ALMA_IMAGE:-almalinux:8}"

if ! command -v docker >/dev/null 2>&1; then
  echo "alma bootstrap: docker is required to build the AlmaLinux rootfs" >&2
  exit 1
fi

docker pull -q "$img" >/dev/null
name="capper-alma-build-$$"
docker rm -f "$name" >/dev/null 2>&1 || true

docker run --name "$name" "$img" bash -c \
  "dnf -y swap coreutils-single coreutils && dnf -y install bash procps-ng util-linux iproute iputils findutils which tar gzip less gawk sed grep && dnf -y clean all && rm -rf /var/cache/dnf /tmp/* /var/log/*" >/dev/null

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

# Login shells source /etc/profile.d before /etc/bashrc; bashrc can reset PS1 for
# the default prompt, so set Capper's prompt again after those defaults run.
if ! grep -q 'Capper login prompt' rootfs/etc/bashrc 2>/dev/null; then
  cat >> rootfs/etc/bashrc <<'EOF'

# Capper login prompt: user@hostname:cwd#
if [ -n "${PS1-}" ]; then
  PS1='\u@\h:\w# '
fi
EOF
fi

if find rootfs -name "*busybox*" 2>/dev/null | grep -q .; then
  echo "alma bootstrap: unexpected busybox in rootfs:" >&2
  find rootfs -name "*busybox*" 2>/dev/null >&2
  exit 1
fi

echo "AlmaLinux rootfs ready at examples/alma/rootfs ($(du -sh rootfs | cut -f1))"
