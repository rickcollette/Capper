#!/usr/bin/env bash
# Installer shipped inside the Capper AIO tarball. Installs into a *versioned*
# layout so `capper aio upgrade` / `--rollback` are atomic symlink flips:
#
#   /usr/local/lib/capper/<version>/{bin,console}   staged release
#   /usr/local/lib/capper/current  -> <version>      active version (symlink)
#   /usr/local/bin/<bin>           -> current/bin/<bin>
#   /opt/capper/console            -> current/console
#
# Re-running on an existing install performs an in-place version add + flip
# (an upgrade) rather than overwriting files.
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
LIB_ROOT="${LIB_ROOT:-$PREFIX/lib/capper}"
CONSOLE_LINK="${CONSOLE_DIR:-/opt/capper/console}"
BACKEND="capdb"
LISTEN="127.0.0.1:8080"
YES=0
SKIP_DEPS=0
SKIP_DOCKER=0
SKIP_COMPOSE=0
CHECK_ONLY=0
DOCTOR_ONLY=0
OFFLINE_DEPS=""

here="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

usage() {
  cat <<EOF
Usage: sudo ./install.sh [options]

Options:
  --yes                  non-interactive; assume yes
  --backend NAME         backend for next-step init hint (default: capdb)
  --listen ADDR          listen address for control-plane drop-in (default: 127.0.0.1:8080)
  --skip-deps            do not install OS dependency packages
  --skip-docker          do not install/start Docker Engine
  --skip-compose         do not install Docker Compose plugin
  --offline-deps DIR     reserved for offline dependency bundles
  --check-only           detect platform and verify prerequisites only
  --doctor-only          run capper aio doctor only
  -h, --help             show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --yes) YES=1 ;;
    --backend) BACKEND="${2:?--backend requires a value}"; shift ;;
    --listen) LISTEN="${2:?--listen requires a value}"; shift ;;
    --skip-deps) SKIP_DEPS=1 ;;
    --skip-docker) SKIP_DOCKER=1 ;;
    --skip-compose) SKIP_COMPOSE=1 ;;
    --offline-deps) OFFLINE_DEPS="${2:?--offline-deps requires a value}"; shift ;;
    --check-only) CHECK_ONLY=1 ;;
    --doctor-only) DOCTOR_ONLY=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

say() { printf '\n==> %s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }
confirm() {
  [ "$YES" = "1" ] && return 0
  printf '%s [y/N] ' "$1"
  read -r ans
  [ "$ans" = "y" ] || [ "$ans" = "Y" ] || [ "$ans" = "yes" ] || [ "$ans" = "YES" ]
}

if [ "$(id -u)" -ne 0 ]; then
  die "run as root (sudo ./install.sh)"
fi

VERSION="$(cat "$here/VERSION" 2>/dev/null || echo unknown)"
DEST="$LIB_ROOT/$VERSION"
MANIFEST_PLATFORM="$(python3 - "$here/manifest.json" 2>/dev/null <<'PY' || true
import json, sys
with open(sys.argv[1]) as f:
    print(json.load(f).get("platform", ""))
PY
)"

detect_platform() {
  ARCH="$(uname -m)"
  [ "$ARCH" = "x86_64" ] || die "unsupported architecture $ARCH; this bundle is x86_64 only"
  . /etc/os-release
  OS_ID="${ID:-unknown}"
  OS_VERSION="${VERSION_ID:-unknown}"
  GLIBC="$(getconf GNU_LIBC_VERSION 2>/dev/null | awk '{print $2}')"
  echo "OS: ${PRETTY_NAME:-$OS_ID $OS_VERSION}"
  echo "Arch: $ARCH"
  echo "glibc: ${GLIBC:-unknown}"
  echo "Bundle platform: ${MANIFEST_PLATFORM:-unknown}"
}

install_apt_deps() {
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y ca-certificates curl tar gzip python3 openssl iproute2 iptables nftables libcap2-bin lvm2 systemd jq util-linux e2fsprogs xfsprogs bubblewrap crun || \
    apt-get install -y ca-certificates curl tar gzip python3 openssl iproute2 iptables libcap2-bin lvm2 systemd jq util-linux e2fsprogs xfsprogs bubblewrap runc
}

install_dnf_deps() {
  dnf install -y ca-certificates curl tar gzip python3 openssl iproute iptables nftables libcap lvm2 systemd jq util-linux e2fsprogs xfsprogs bubblewrap crun || \
    dnf install -y ca-certificates curl tar gzip python3 openssl iproute iptables libcap lvm2 systemd jq util-linux e2fsprogs xfsprogs bubblewrap runc
}

install_docker_apt() {
  if have docker; then
    echo "Docker already installed: $(docker --version)"
    return
  fi
  if confirm "Install Docker Engine from Docker's apt repository?"; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/${OS_ID}/gpg" -o /etc/apt/keyrings/docker.asc || die "failed to fetch Docker apt key"
    chmod a+r /etc/apt/keyrings/docker.asc
    codename="${VERSION_CODENAME:-}"
    [ -n "$codename" ] || codename="$(. /etc/os-release; echo "${UBUNTU_CODENAME:-}")"
    [ -n "$codename" ] || die "cannot determine apt codename for Docker repository"
    echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${OS_ID} ${codename} stable" > /etc/apt/sources.list.d/docker.list
    apt-get update
    docker_pkgs=(docker-ce docker-ce-cli containerd.io docker-buildx-plugin)
    [ "$SKIP_COMPOSE" = "1" ] || docker_pkgs+=(docker-compose-plugin)
    apt-get install -y "${docker_pkgs[@]}"
  else
    apt-get install -y docker.io docker-compose-plugin || apt-get install -y docker.io docker-compose
  fi
}

install_docker_dnf() {
  if have docker; then
    echo "Docker already installed: $(docker --version)"
    return
  fi
  if confirm "Install Docker Engine from Docker's dnf repository?"; then
    dnf install -y dnf-plugins-core
    dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    docker_pkgs=(docker-ce docker-ce-cli containerd.io docker-buildx-plugin)
    [ "$SKIP_COMPOSE" = "1" ] || docker_pkgs+=(docker-compose-plugin)
    dnf install -y "${docker_pkgs[@]}"
  else
    dnf install -y docker docker-compose-plugin || dnf install -y podman podman-compose
  fi
}

install_deps() {
  [ "$SKIP_DEPS" = "1" ] && return
  [ -n "$OFFLINE_DEPS" ] && warn "--offline-deps is reserved; falling back to configured OS repositories"
  if have apt-get; then
    install_apt_deps
  elif have dnf; then
    install_dnf_deps
  else
    die "unsupported package manager; expected apt-get or dnf"
  fi
}

install_docker_stack() {
  [ "$SKIP_DOCKER" = "1" ] && return
  if have apt-get; then
    install_docker_apt
  elif have dnf; then
    install_docker_dnf
  fi
  if have systemctl; then
    systemctl enable --now docker 2>/dev/null || true
  fi
  docker version >/dev/null 2>&1 || warn "docker version did not pass; check Docker service status"
  if [ "$SKIP_COMPOSE" != "1" ]; then
    docker compose version >/dev/null 2>&1 || warn "docker compose version did not pass; install docker-compose-plugin"
  fi
}

say "Platform preflight"
detect_platform

if [ "$DOCTOR_ONLY" = "1" ]; then
  "$PREFIX/bin/capper" aio doctor
  exit $?
fi

if [ "$CHECK_ONLY" = "1" ]; then
  have systemctl || warn "systemctl not found"
  have docker || warn "docker not found"
  docker compose version >/dev/null 2>&1 || warn "docker compose not found"
  exit 0
fi

say "Installing host dependencies"
install_deps
install_docker_stack

prev=""
if [ -L "$LIB_ROOT/current" ]; then
  prev="$(basename "$(readlink "$LIB_ROOT/current")")"
  echo "Existing install detected (version $prev) — staging $VERSION as an upgrade."
fi

echo "Staging release $VERSION -> $DEST"
install -d -m 0755 "$LIB_ROOT"
rm -rf "$DEST"
install -d -m 0755 "$DEST/bin"
for b in capper capper-agent capinit capdb-server; do
  [ -f "$here/bin/$b" ] && install -m 0755 "$here/bin/$b" "$DEST/bin/$b"
done
if [ -d "$here/console" ]; then
  install -d -m 0755 "$DEST/console"
  cp -a "$here/console/." "$DEST/console/"
fi

# Flip the active version atomically.
ln -sfn "$DEST" "$LIB_ROOT/current.tmp"
mv -Tf "$LIB_ROOT/current.tmp" "$LIB_ROOT/current"

# Point stable paths through current/.
install -d -m 0755 "$PREFIX/bin"
for b in capper capper-agent capinit capdb-server; do
  [ -f "$LIB_ROOT/current/bin/$b" ] && ln -sfn "$LIB_ROOT/current/bin/$b" "$PREFIX/bin/$b"
done
if [ -d "$LIB_ROOT/current/console" ]; then
  install -d -m 0755 "$(dirname "$CONSOLE_LINK")"
  ln -sfn "$LIB_ROOT/current/console" "$CONSOLE_LINK"
fi

# Drop-in so the control plane serves the console (the base unit has no --console).
if command -v systemctl >/dev/null 2>&1; then
  install -d -m 0755 /etc/systemd/system/capper-control.service.d
  cat > /etc/systemd/system/capper-control.service.d/10-console.conf <<EOF
[Service]
ExecStart=
ExecStart=$PREFIX/bin/capper api start --listen $LISTEN --console $CONSOLE_LINK
EOF
  systemctl daemon-reload || true
fi

if [ -n "$prev" ]; then
  cat <<EOF

Upgraded $prev -> $VERSION. Apply it with:

  sudo capper aio upgrade --bundle <this-bundle>.tgz   # full orchestrated upgrade
  # or, since binaries are already staged, restart services:
  sudo systemctl restart capdb-server capper-control capper-agent

Roll back with: sudo capper aio upgrade --rollback
EOF
else
  cat <<EOF

Capper AIO $VERSION installed. Next steps:

  sudo capper aio doctor
  sudo capper aio init --backend $BACKEND
  sudo capper aio up
  capper aio status

The console is served by the control plane on http://$LISTEN.
Use 'capper aio doctor' for pre-flight checks and 'capper aio down' to stop.
EOF
fi
