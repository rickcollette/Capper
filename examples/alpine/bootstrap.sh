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

# free(1) that reads /proc/meminfo (which Capper masks per-instance) instead of
# the sysinfo() syscall busybox uses — so memory reflects the capsule's limit,
# not the host. Replace the busybox 'free' symlink (-> /bin/busybox) with a real
# file; rm -f first so we don't write THROUGH the symlink.
rm -f rootfs/usr/bin/free rootfs/bin/free
cat > rootfs/usr/bin/free <<'FREE'
#!/bin/sh
mode=k
for a in "$@"; do
  case "$a" in
    -h|--human) mode=h ;;
    -b|--bytes) mode=b ;;
    -k|--kilo)  mode=k ;;
    -m|--mega)  mode=m ;;
    -g|--giga)  mode=g ;;
  esac
done
awk -v mode="$mode" '
function fmt(kb,   b,u) {
  if (mode=="k") return sprintf("%d", kb)
  if (mode=="m") return sprintf("%d", kb/1024)
  if (mode=="g") return sprintf("%d", kb/1048576)
  if (mode=="b") return sprintf("%d", kb*1024)
  b=kb*1024; u="B"
  if (b>=1024){b/=1024;u="K"}
  if (b>=1024){b/=1024;u="M"}
  if (b>=1024){b/=1024;u="G"}
  if (b>=1024){b/=1024;u="T"}
  return sprintf("%.1f%s", b, u)
}
/^MemTotal:/     {t=$2}
/^MemFree:/      {f=$2}
/^MemAvailable:/ {av=$2}
/^Buffers:/      {bu=$2}
/^Cached:/       {ca=$2}
/^SwapTotal:/    {st=$2}
/^SwapFree:/     {sf=$2}
END {
  used=t-f-bu-ca; if (used<0) used=0
  printf "%-10s %12s %12s %12s %12s %12s %12s\n","","total","used","free","shared","buff/cache","available"
  printf "%-10s %12s %12s %12s %12s %12s %12s\n","Mem:",fmt(t),fmt(used),fmt(f),fmt(0),fmt(bu+ca),fmt(av)
  printf "%-10s %12s %12s %12s\n","Swap:",fmt(st),fmt(st-sf),fmt(sf)
}' /proc/meminfo
FREE
chmod 0755 rootfs/usr/bin/free
cp rootfs/usr/bin/free rootfs/bin/free

echo "Alpine rootfs ready at examples/alpine/rootfs (${tarball})"
