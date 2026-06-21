#!/usr/bin/env bash
# deploy-local.sh — Local build wrapper with cleanup
#
# This script:
#   1. Cleans old deploy files (DIST/AIO)
#   2. Builds new deploy package (if cmake available)
#   3. Validates the build
#   4. Prepares for deployment (deploy/deploy.sh will handle remote)
#
# Usage:
#   bash deploy-local.sh [--clean-only] [--skip-tests]
#
set -euo pipefail

HERE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$HERE"

# ── Configuration ─────────────────────────────────────────────────────────────
CLEAN_ONLY="${CLEAN_ONLY:-0}"
SKIP_TESTS="${SKIP_TESTS:-0}"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --clean-only) CLEAN_ONLY=1; shift ;;
    --skip-tests) SKIP_TESTS=1; shift ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# ── Colors ────────────────────────────────────────────────────────────────────
if [ -t 1 ]; then
  C='\033[1;36m'
  G='\033[1;32m'
  R='\033[1;31m'
  Y='\033[1;33m'
  Z='\033[0m'
else
  C='' G='' R='' Y='' Z=''
fi

say()  { printf "\n${C}==> %s${Z}\n" "$*"; }
ok()   { printf "${G}  ✓ %s${Z}\n" "$*"; }
warn() { printf "${Y}  ! %s${Z}\n" "$*"; }
die()  { printf "${R}  ✗ %s${Z}\n" "$*" >&2; exit 1; }

# ── 1. Cleanup ────────────────────────────────────────────────────────────────
say "Cleaning old deploy files"

DIST_DIR="DIST/AIO"
if [ -d "$DIST_DIR" ]; then
  rm -rf "$DIST_DIR/stage" "$DIST_DIR"/*.tgz "$DIST_DIR"/*.sha256
  ok "Cleaned $DIST_DIR"
else
  ok "No previous build artifacts to clean"
fi

if [ "$CLEAN_ONLY" = "1" ]; then
  say "Clean-only mode — exiting"
  exit 0
fi

# ── 2. Preflight checks ───────────────────────────────────────────────────────
say "Preflight checks"

# Check required tools for building
for tool in go git tar; do
  if command -v "$tool" >/dev/null 2>&1; then
    echo "  ✓ $tool: $(command -v $tool)"
  else
    die "missing required tool: $tool"
  fi
done

# Check cmake (needed for CapDB)
if command -v cmake >/dev/null 2>&1; then
  echo "  ✓ cmake: $(cmake --version | head -1)"
  CAN_BUILD=1
elif [ -n "${CMAKE_PATH:-}" ] && [ -x "$CMAKE_PATH" ]; then
  export PATH="$CMAKE_PATH:$PATH"
  echo "  ✓ cmake: using CMAKE_PATH=$CMAKE_PATH"
  CAN_BUILD=1
else
  warn "cmake not found"
  CAN_BUILD=0
fi

# Check gcc/cc (needed for CapDB)
if command -v gcc >/dev/null 2>&1; then
  echo "  ✓ gcc: $(gcc --version | head -1)"
elif command -v cc >/dev/null 2>&1; then
  echo "  ✓ cc: $(cc --version | head -1)"
else
  if [ "$CAN_BUILD" = "1" ]; then
    warn "gcc/cc not found (needed for CapDB)"
    CAN_BUILD=0
  fi
fi

# CapperWeb not required for backend-only build
if [ -d "/home/megalith/CapperWeb" ]; then
  echo "  ✓ CapperWeb: found at /home/megalith/CapperWeb"
else
  warn "CapperWeb not found at /home/megalith/CapperWeb"
fi

# ── 3. Version management ─────────────────────────────────────────────────────
say "Version management"

if [ -f "VERSION" ]; then
  CURRENT_VERSION="$(tr -d ' \n\r' < VERSION)"
  echo "  current: $CURRENT_VERSION"
else
  CURRENT_VERSION="0.0.0-$(date +%Y%m%d)"
  echo "  current: $CURRENT_VERSION (generated)"
fi

# ── 4. Build decision ─────────────────────────────────────────────────────────
say "Build decision"

if [ "$CAN_BUILD" = "1" ]; then
  echo "  All requirements met — building"

  say "Building deployment package"
  echo "  Version: $CURRENT_VERSION"
  echo "  Skip tests: $SKIP_TESTS"

  # Run the actual build
  SKIP_TESTS="$SKIP_TESTS" scripts/build-aio.sh "$CURRENT_VERSION" || die "Build failed"

  # Verify output
  PKG="capper-aio-${CURRENT_VERSION}-linux-amd64"
  TGZ="DIST/AIO/${PKG}.tgz"

  if [ -f "$TGZ" ]; then
    SIZE=$(du -h "$TGZ" | cut -f1)
    ok "Built: $TGZ ($SIZE)"
    echo ""
    say "Build successful!"
    echo "  Package: $TGZ"
    echo "  Size: $SIZE"
    echo ""
    echo "Next: bash deploy/deploy.sh"
    exit 0
  else
    die "Build completed but tarball not found at $TGZ"
  fi

else
  # Provide helpful instructions for missing dependencies
  echo ""
  echo "  Missing: cmake, gcc/cc"
  echo ""
  echo "  To fix, install build tools on your build machine:"
  echo ""
  echo "    sudo apt-get update"
  echo "    sudo apt-get install -y cmake build-essential"
  echo ""
  echo "  Then retry:"
  echo ""
  echo "    bash deploy-local.sh"
  echo ""
  echo "  Or, if you have cmake/gcc elsewhere, set CMAKE_PATH:"
  echo ""
  echo "    CMAKE_PATH=/path/to/cmake bash deploy-local.sh"
  echo ""
  die "Cannot build without cmake and gcc"
fi
