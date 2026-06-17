#!/bin/bash
# Build the ShipLog Unraid plugin package (.txz) = the engine binary + plugin
# files. Portable (uses tar, not Slackware makepkg) so it runs in GitHub CI.
#
#   plugin/pkg_build.sh [VERSION]      # VERSION defaults to today (YYYY.MM.DD)
#
# Output: plugin/out/shiplog-<version>-x86_64-1.txz (+ .sha256). The release
# workflow attaches the .txz and injects the SHA256 into shiplog.plg.
set -euo pipefail

VERSION="${1:-$(date +%Y.%m.%d)}"
ARCH="x86_64"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PLUGIN_SRC="$ROOT/plugin/src/shiplog"
BIN_REL="usr/local/emhttp/plugins/shiplog/bin/shiplog"
OUT="$ROOT/plugin/out"
PKGROOT="$(mktemp -d)"
trap 'rm -rf "$PKGROOT"' EXIT

echo "==> building engine binary (linux/amd64)"
( cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" -o "$PLUGIN_SRC/$BIN_REL" ./cmd/shiplog )

echo "==> assembling package tree"
cp -a "$PLUGIN_SRC/." "$PKGROOT/"
chmod +x "$PKGROOT/usr/local/emhttp/plugins/shiplog/scripts/rc.shiplog"
chmod +x "$PKGROOT/usr/local/emhttp/plugins/shiplog/event/"* 2>/dev/null || true
chmod +x "$PKGROOT/$BIN_REL"

mkdir -p "$OUT"
TXZ="$OUT/shiplog-$VERSION-$ARCH-1.txz"
echo "==> packaging → $TXZ"
# --force-local: a Windows output path like "D:/..." has a colon that GNU tar
# would otherwise read as a remote host[:path]. Harmless on Linux/CI.
tar --force-local -C "$PKGROOT" -caf "$TXZ" .

echo "==> sha256"
sha256sum "$TXZ" | tee "$TXZ.sha256"
echo "done: $TXZ"
