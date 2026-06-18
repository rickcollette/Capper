#!/bin/sh
set -eu

arch="x86_64"
base_url="https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/${arch}"

cd "$(dirname "$0")"
mkdir -p downloads

# Discover the actual minirootfs tarball in latest-stable rather than pinning a
# version (the pinned file often 404s when latest-stable advances). Allow an
# override via ALPINE_TARBALL for reproducible/offline builds.
tarball="${ALPINE_TARBALL:-}"
if [ -z "$tarball" ]; then
  tarball="$(curl -fsSL "${base_url}/" \
    | grep -oE "alpine-minirootfs-[0-9.]+-${arch}\.tar\.gz" \
    | sort -V | tail -1)"
fi
if [ -z "$tarball" ]; then
  echo "bootstrap: could not determine alpine minirootfs filename from ${base_url}" >&2
  exit 1
fi
sha512="${tarball}.sha512"

if [ ! -f "downloads/${tarball}" ]; then
  curl -fL -o "downloads/${tarball}" "${base_url}/${tarball}"
fi
if [ ! -f "downloads/${sha512}" ]; then
  curl -fL -o "downloads/${sha512}" "${base_url}/${sha512}"
fi

(cd downloads && sha512sum -c "${sha512}")

# Clean extract so a previous partial/empty rootfs can't be packaged.
rm -rf rootfs
mkdir -p rootfs
tar -xzf "downloads/${tarball}" -C rootfs

# Baseline shell environment for interactive (login) shells: PATH and a prompt
# of the form user@host:cwd# . Hostname is set per-instance by capinit on boot.
mkdir -p rootfs/etc rootfs/sbin
cat > rootfs/etc/profile <<'PROF'
export PATH=/bin:/sbin:/usr/bin:/usr/sbin
export PS1='\u@\h:\w# '
PROF

echo "Alpine rootfs ready at examples/alpine/rootfs (${tarball})"
