#!/bin/sh
# Build a busybox-free Alpine rootfs for Capper capsules.
# Installs full GNU/core/procps/util-linux tooling, then purges busybox entirely.
# Requires docker on the build host (same pattern as examples/alma).
set -eu

cd "$(dirname "$0")"
img="${ALPINE_IMAGE:-alpine:3.20}"

if ! command -v docker >/dev/null 2>&1; then
  echo "alpine bootstrap: docker is required to build the Alpine rootfs" >&2
  exit 1
fi

docker pull -q "$img" >/dev/null
name="capper-alpine-build-$$"
docker rm -f "$name" >/dev/null 2>&1 || true

docker run --name "$name" "$img" sh -c '
  set -eu
  apk add --no-cache \
    bash coreutils procps-ng util-linux shadow findutils grep sed gawk tar gzip which less \
    ca-certificates iproute2 alpine-release apk-tools

  ln -sf bash /bin/sh

  # Remove busybox binary and metadata.
  rm -f /bin/busybox /usr/bin/busybox
  rm -rf /etc/busybox-paths.d

  # Drop symlinks that still point at busybox.
  for link in $(find /bin /sbin /usr/bin /usr/sbin -type l 2>/dev/null); do
    target=$(readlink "$link" 2>/dev/null || true)
    case "$target" in *busybox*|busybox) rm -f "$link" ;; esac
  done

  # Restore /bin names for tools that live under /usr/bin after the purge.
  for cmd in find sed awk grep gzip tar ps free top which less; do
    if [ ! -e "/bin/$cmd" ]; then
      if [ -x "/usr/bin/$cmd" ]; then
        ln -sf "../usr/bin/$cmd" "/bin/$cmd"
      elif [ -x "/bin/coreutils" ]; then
        ln -sf coreutils "/bin/$cmd" 2>/dev/null || true
      fi
    fi
  done

  if find / -name "*busybox*" 2>/dev/null | grep -q .; then
    echo "bootstrap: busybox artifacts remain:" >&2
    find / -name "*busybox*" 2>/dev/null >&2
    exit 1
  fi

  mkdir -p /etc/profile.d
  cat > /etc/profile.d/capper-tools.sh <<EOF
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EOF
  cat > /etc/profile <<EOF
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PS1='\u@\h:\w# '
EOF
' >/dev/null

rm -rf rootfs && mkdir rootfs
docker export "$name" | tar -x -C rootfs 2>/dev/null
docker rm -f "$name" >/dev/null

chmod -R u+rwX rootfs

# apk DB still lists busybox packages from the transient install layer; scrub
# those records so the shipped rootfs does not advertise busybox.
if [ -f rootfs/lib/apk/db/installed ]; then
  awk '
    /^P:busybox/ { skip=1; next }
    /^P:busybox-binsh/ { skip=1; next }
    /^P:/ { skip=0 }
    !skip { print }
  ' rootfs/lib/apk/db/installed > rootfs/lib/apk/db/installed.tmp
  mv rootfs/lib/apk/db/installed.tmp rootfs/lib/apk/db/installed
fi

if find rootfs -name "*busybox*" 2>/dev/null | grep -q .; then
  echo "bootstrap: busybox still present in exported rootfs:" >&2
  find rootfs -name "*busybox*" 2>/dev/null >&2
  exit 1
fi

echo "Alpine busybox-free rootfs ready at examples/alpine/rootfs ($(du -sh rootfs | cut -f1))"
