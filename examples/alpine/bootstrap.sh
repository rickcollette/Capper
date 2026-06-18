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

  # Remove busybox the apk-native way so /etc/apk/world AND the installed DB stay
  # consistent. This is what lets `apk add` work later in the capsule: otherwise
  # world still lists busybox and every add aborts with "busybox (no such
  # package)". busybox-binsh owns /bin/sh, so recreate it as a bash symlink.
  # apk-tools 2.14 does its own TLS (OpenSSL), so this does not break apk over
  # https. alpine-base is the meta-package that pulls busybox in.
  apk del --no-cache busybox-suid busybox-binsh busybox alpine-base 2>/dev/null || true
  ln -sf bash /bin/sh

  # Belt-and-suspenders: scrub any busybox left in WORLD even if apk could not
  # resolve the removal cleanly on this Alpine version.
  if [ -f /etc/apk/world ]; then
    grep -vE "^(busybox|busybox-binsh|busybox-suid|busybox-extras|alpine-base)$" \
      /etc/apk/world > /etc/apk/world.tmp || true
    mv /etc/apk/world.tmp /etc/apk/world
  fi

  # Purge any busybox binary/metadata apk del missed.
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
  ln -sf bash /bin/sh

  if find / -name "*busybox*" 2>/dev/null | grep -q .; then
    echo "bootstrap: busybox artifacts remain:" >&2
    find / -name "*busybox*" 2>/dev/null >&2
    exit 1
  fi
' >/dev/null

rm -rf rootfs && mkdir rootfs
docker export "$name" | tar -x -C rootfs 2>/dev/null
docker rm -f "$name" >/dev/null

chmod -R u+rwX rootfs

# Write profile snippets from the host so PS1 backslashes are not eaten by sh
# heredocs inside the docker build container.
mkdir -p rootfs/etc/profile.d
cat > rootfs/etc/profile.d/capper-tools.sh <<'EOF'
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EOF
cat > rootfs/etc/profile.d/capper-prompt.sh <<'EOF'
# Capper login prompt: user@hostname:cwd#
export PS1='\u@\h:\w# '
EOF

# Final guarantee on the exported rootfs: WORLD must not pull busybox back in.
if [ -f rootfs/etc/apk/world ]; then
  grep -vE '^(busybox|busybox-binsh|busybox-suid|busybox-extras|alpine-base)$' \
    rootfs/etc/apk/world > rootfs/etc/apk/world.tmp || true
  mv rootfs/etc/apk/world.tmp rootfs/etc/apk/world
fi

# Scrub any busybox/alpine-base records the apk del above could not remove, using
# paragraph mode so whole package blocks (and their blank separators) drop cleanly.
if [ -f rootfs/lib/apk/db/installed ]; then
  awk 'BEGIN { RS=""; ORS="\n\n" }
       !/(^|\n)P:(busybox|busybox-binsh|busybox-suid|busybox-extras|alpine-base)\n/' \
    rootfs/lib/apk/db/installed > rootfs/lib/apk/db/installed.tmp
  mv rootfs/lib/apk/db/installed.tmp rootfs/lib/apk/db/installed

  # Register bash as the provider of /bin/sh + cmd:sh so that packages declaring a
  # shell dependency still resolve now that busybox-binsh (the old provider) is
  # gone — without this, `apk add <pkg-needing-sh>` fails with "cmd:sh (no such
  # package)".
  awk '
    /^P:bash$/ { inbash=1 }
    inbash && /^p:/ { if ($0 !~ /cmd:sh/) $0 = $0 " /bin/sh=0 cmd:sh=0"; inbash=0 }
    /^$/ { inbash=0 }
    { print }
  ' rootfs/lib/apk/db/installed > rootfs/lib/apk/db/installed.tmp
  mv rootfs/lib/apk/db/installed.tmp rootfs/lib/apk/db/installed
fi

if find rootfs -name "*busybox*" 2>/dev/null | grep -q .; then
  echo "bootstrap: busybox still present in exported rootfs:" >&2
  find rootfs -name "*busybox*" 2>/dev/null >&2
  exit 1
fi

echo "Alpine busybox-free rootfs ready at examples/alpine/rootfs ($(du -sh rootfs | cut -f1))"
