#!/bin/sh
# Build an AlmaLinux rootfs for the sample image by exporting the official
# container image with a full core OS toolset (no busybox — Alma never ships it).
# Requires docker on the build host.
set -eu

cd "$(dirname "$0")"
img="${ALMA_IMAGE:-almalinux:9}"

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
printf "export PS1='\\u@\\h:\\w# '\n" > rootfs/etc/profile.d/capper-prompt.sh

if find rootfs -name "*busybox*" 2>/dev/null | grep -q .; then
  echo "alma bootstrap: unexpected busybox in rootfs:" >&2
  find rootfs -name "*busybox*" 2>/dev/null >&2
  exit 1
fi

echo "AlmaLinux rootfs ready at examples/alma/rootfs ($(du -sh rootfs | cut -f1))"
