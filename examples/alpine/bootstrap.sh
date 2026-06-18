#!/bin/sh
# Build a full-toolset Alpine rootfs for the sample image using apk.static, so
# capsules get real GNU/util-linux tooling (bash, coreutils, procps, util-linux)
# instead of busybox applets. procps' free(1) reads /proc/meminfo (which Capper
# masks per-instance), so it reports the capsule's memory, not the host's.
set -eu

arch="x86_64"
base="https://dl-cdn.alpinelinux.org/alpine/latest-stable"
pkgs="alpine-base bash coreutils procps-ng util-linux shadow findutils grep sed ca-certificates iproute2"

cd "$(dirname "$0")"
mkdir -p downloads

SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO="sudo"

# Fetch the static apk-tools (lets us build a rootfs from any host, no chroot).
apkpkg="$(curl -fsSL "$base/main/$arch/" \
  | grep -oE 'apk-tools-static-[0-9._r-]+\.apk' | sort -V | tail -1)"
[ -n "$apkpkg" ] || { echo "bootstrap: cannot find apk-tools-static" >&2; exit 1; }
if [ ! -f "downloads/$apkpkg" ]; then
  curl -fsSL "$base/main/$arch/$apkpkg" -o "downloads/$apkpkg"
fi
rm -rf apktools && mkdir apktools
tar -xzf "downloads/$apkpkg" -C apktools 2>/dev/null
apk="apktools/sbin/apk.static"
[ -x "$apk" ] || { echo "bootstrap: apk.static not found in $apkpkg" >&2; exit 1; }

# Clean build so a previous rootfs can't leak in.
$SUDO rm -rf rootfs
mkdir -p rootfs
$SUDO "$apk" -X "$base/main" -X "$base/community" -U --allow-untrusted \
  --root rootfs --initdb add $pkgs

# Hand the tree back to the build user so the (non-root) packaging steps
# (capinit copy, capper create) can read/modify it. apk leaves some setuid
# binaries mode ---x--x--x; capper create must be able to read every file.
$SUDO chown -R "$(id -u):$(id -g)" rootfs
$SUDO chmod -R u+rwX rootfs

# Baseline login-shell environment: PATH and a user@host:cwd# prompt. Hostname
# is set per-instance by capinit on boot.
mkdir -p rootfs/etc
cat > rootfs/etc/profile <<'PROF'
export PATH=/bin:/sbin:/usr/bin:/usr/sbin
export PS1='\u@\h:\w# '
PROF

rm -rf apktools
echo "Alpine full-toolset rootfs ready at examples/alpine/rootfs ($(du -sh rootfs | cut -f1))"
