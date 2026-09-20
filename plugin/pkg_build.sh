#!/bin/bash
# Builds the Unraid plugin package (.txz) from the engine binary and the plugin
# files. It uses tar instead of Slackware's makepkg, so it runs in CI.
#
#   plugin/pkg_build.sh [VERSION]      # VERSION defaults to today (YYYY.MM.DD)
#
# Output: plugin/out/shiplog-<version>-x86_64-1.txz and its .sha256.
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

# Unraid's PageBuilder drops a CRLF .page, and a trailing CR breaks shebangs.
# .gitattributes covers checkouts; this covers a Windows autocrlf tree.
echo "==> normalising text files to LF"
find "$PKGROOT" -type f ! -path "*/bin/*" ! -name '*.png' -print0 \
  | while IFS= read -r -d '' f; do perl -i -pe 's/\r\n/\n/g; s/\r$//' "$f"; done

mkdir -p "$OUT"
TXZ="$OUT/shiplog-$VERSION-$ARCH-1.txz"
echo "==> packaging → $TXZ"
# --force-local keeps GNU tar from reading the colon in "D:/..." as a remote
# host. Every entry, "./" included, has to be root:root, because upgradepkg
# applies the owner of "./" to / and a wrong owner there breaks sshd key auth.
tar --force-local --owner=0 --group=0 --numeric-owner -C "$PKGROOT" -caf "$TXZ" .

echo "==> sha256"
# A bare filename in the .sha256 lets `sha256sum -c` work after a download.
( cd "$OUT" && b="$(basename "$TXZ")" && sha256sum "$b" | tee "$b.sha256" )
echo "done: $TXZ"
