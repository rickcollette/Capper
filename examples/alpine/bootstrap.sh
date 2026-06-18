#!/bin/sh
set -eu

version="3.23.0"
arch="x86_64"
base_url="https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/${arch}"
tarball="alpine-minirootfs-${version}-${arch}.tar.gz"
sha512="${tarball}.sha512"

cd "$(dirname "$0")"
mkdir -p downloads rootfs

if [ ! -f "downloads/${tarball}" ]; then
  curl -L -o "downloads/${tarball}" "${base_url}/${tarball}"
fi

if [ ! -f "downloads/${sha512}" ]; then
  curl -L -o "downloads/${sha512}" "${base_url}/${sha512}"
fi

(cd downloads && sha512sum -c "${sha512}")
tar -xzf "downloads/${tarball}" -C rootfs

echo "Alpine ${version} rootfs is ready at examples/alpine/rootfs"
