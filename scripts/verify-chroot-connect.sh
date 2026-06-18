#!/usr/bin/env bash
# Manual verification script for chroot connect behavior.
#
# Prerequisites:
#   - Run as root (chroot requires CAP_SYS_CHROOT)
#   - /bin/busybox must exist as a static binary
#   - capper binary must be built and available as ./bin/capper
#
# Usage:
#   sudo bash scripts/verify-chroot-connect.sh
set -euo pipefail

CAPPER=${CAPPER:-./bin/capper}
STORE=$(mktemp -d)
WORKDIR=$(mktemp -d)
trap 'rm -rf "$STORE" "$WORKDIR"' EXIT

if [ "$(id -u)" != "0" ]; then
  echo "ERROR: chroot connect requires root. Re-run with sudo." >&2
  exit 1
fi

if [ ! -f /bin/busybox ]; then
  echo "ERROR: /bin/busybox not found. Install a static busybox." >&2
  exit 1
fi

echo "==> Building test rootfs..."
ROOTFS="$WORKDIR/rootfs"
mkdir -p "$ROOTFS/bin"
cp /bin/busybox "$ROOTFS/bin/sh"
chmod 755 "$ROOTFS/bin/sh"

echo "==> Writing capper.json..."
cat > "$WORKDIR/capper.json" <<'EOF'
{
  "name": "verify-connect",
  "version": "0.1.0",
  "rootfs": "./rootfs",
  "entrypoint": ["/bin/sh"],
  "args": ["-c", "sleep 30"]
}
EOF

echo "==> Creating image..."
"$CAPPER" --store "$STORE" --runtime chroot create verify-connect.cap "$WORKDIR/capper.json"

echo "==> Running instance (background)..."
INSTANCE=$("$CAPPER" --store "$STORE" --runtime chroot --json run verify-connect.cap | python3 -c "import json,sys; print(json.load(sys.stdin)['name'])")
echo "    Instance: $INSTANCE"

echo "==> Waiting for instance to start..."
sleep 1

echo "==> Verifying instance is running..."
STATUS=$("$CAPPER" --store "$STORE" --json inspect instance "$INSTANCE" | python3 -c "import json,sys; print(json.load(sys.stdin)['status'])")
if [ "$STATUS" != "running" ]; then
  echo "ERROR: Expected status=running, got $STATUS" >&2
  "$CAPPER" --store "$STORE" logs "$INSTANCE" >&2
  exit 1
fi

echo "==> Connecting with chroot (runs 'id' non-interactively via exec)..."
# Use capper exec instead of connect for non-interactive verification
OUTPUT=$("$CAPPER" --store "$STORE" --runtime chroot exec "$INSTANCE" /bin/sh -c "id && echo chroot-connect-ok")
if echo "$OUTPUT" | grep -q "chroot-connect-ok"; then
  echo "    PASS: chroot exec works, output: $OUTPUT"
else
  echo "ERROR: Unexpected exec output: $OUTPUT" >&2
  exit 1
fi

echo "==> Stopping instance..."
"$CAPPER" --store "$STORE" stop "$INSTANCE"

echo "==> Cleaning up..."
"$CAPPER" --store "$STORE" rm "$INSTANCE"
"$CAPPER" --store "$STORE" delete verify-connect.cap

echo ""
echo "All chroot connect checks PASSED."
