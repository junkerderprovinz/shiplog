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

# Normalise text files to LF. A CRLF .page breaks Unraid's PageBuilder (it splits
# the header on a pure-LF "\n---\n", so a CRLF page never parses and is dropped),
# and a trailing CR breaks shell shebangs. Belt-and-suspenders next to
# .gitattributes, so a Windows/autocrlf checkout still produces a valid package.
echo "==> normalising text files to LF"
find "$PKGROOT" -type f ! -path "*/bin/*" ! -name '*.png' -print0 \
  | while IFS= read -r -d '' f; do perl -i -pe 's/\r\n/\n/g; s/\r$//' "$f"; done

mkdir -p "$OUT"
TXZ="$OUT/shiplog-$VERSION-$ARCH-1.txz"
echo "==> packaging → $TXZ"
# --force-local: a Windows output path like "D:/..." has a colon that GNU tar
# would otherwise read as a remote host[:path]. Harmless on Linux/CI.
# --owner/--group/--numeric-owner: force root:root on every entry INCLUDING
# "./". Without this the builder's uid is baked in, and upgradepkg (running as
# root) applies it to / on install, which breaks sshd StrictModes key auth
# ("bad ownership or modes for directory /"). Happened with the 1.2.3 txz
# built on Windows (MSYS uid 197608).
tar --force-local --owner=0 --group=0 --numeric-owner -C "$PKGROOT" -caf "$TXZ" .

echo "==> sha256"
# cd into $OUT so the .sha256 carries a bare filename, not the build path —
# otherwise `sha256sum -c` fails for anyone who downloads it.
( cd "$OUT" && b="$(basename "$TXZ")" && sha256sum "$b" | tee "$b.sha256" )
echo "done: $TXZ"
