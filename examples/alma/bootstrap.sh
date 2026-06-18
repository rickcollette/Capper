#!/bin/sh
# Build an AlmaLinux rootfs for the sample image by exporting the official
# container image (adds procps-ng so free(1) reads /proc/meminfo, plus basic
# net tools). Requires docker on the build host.
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
# Install a baseline toolset, then strip caches to keep the rootfs lean.
docker run --name "$name" "$img" bash -c \
  "dnf -y install procps-ng iproute iputils && dnf -y clean all && rm -rf /var/cache/dnf /tmp/* /var/log/*" >/dev/null

rm -rf rootfs && mkdir rootfs
docker export "$name" | tar -x -C rootfs 2>/dev/null
docker rm -f "$name" >/dev/null

# Ensure the build user can read/modify the tree (some files, e.g. /etc/gshadow,
# ship mode 0000) so packaging (capinit install, capper create) can read it.
chmod -R u+rwX rootfs

# Login-shell prompt (Alma sources /etc/profile.d/*.sh). Hostname is set per
# instance by capinit on boot.
mkdir -p rootfs/etc/profile.d
printf "export PS1='\\\\u@\\\\h:\\\\w# '\n" > rootfs/etc/profile.d/capper-prompt.sh

echo "AlmaLinux rootfs ready at examples/alma/rootfs ($(du -sh rootfs | cut -f1))"
