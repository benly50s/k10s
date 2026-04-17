#!/usr/bin/env bash
# Packages k10s.exe + bundled deps into a single NSIS installer.
#
# Usage:
#   scripts/build-windows-installer.sh <VERSION>
#
# Prerequisites (caller is responsible):
#   - dist/k10s_windows_amd64_v1/k10s.exe built by goreleaser (or by this
#     script via `go build` when the directory is missing).
#   - makensis on PATH (apt-get install -y nsis on Ubuntu, brew install
#     makensis on macOS).

set -euo pipefail

if [ "${1:-}" = "" ]; then
  echo "usage: $0 <VERSION>" >&2
  exit 2
fi
VERSION="$1"
# Strip a leading 'v' (e.g. v0.1.6 -> 0.1.6) so the installer filename stays
# clean. Upstream Git tags keep the 'v'.
VERSION="${VERSION#v}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

K10S_BIN="dist/k10s_windows_amd64_v1/k10s.exe"

if [ ! -f "$K10S_BIN" ]; then
  echo "==> $K10S_BIN missing, building via go"
  mkdir -p "$(dirname "$K10S_BIN")"
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$K10S_BIN" .
fi

echo "==> fetching windows deps"
bash "$SCRIPT_DIR/fetch-windows-deps.sh"

echo "==> running makensis (VERSION=$VERSION)"
command -v makensis >/dev/null 2>&1 || {
  echo "makensis not found. Install with:"
  echo "  Ubuntu/Debian: sudo apt-get install -y nsis"
  echo "  macOS:         brew install makensis"
  exit 1
}
makensis -V2 "-DVERSION=$VERSION" "-DROOT=$REPO_ROOT" "$SCRIPT_DIR/k10s-installer.nsi"

OUT="dist/k10s-setup-$VERSION.exe"
[ -f "$OUT" ] || { echo "expected $OUT not produced by makensis" >&2; exit 1; }

echo "==> generating sha256"
if command -v sha256sum >/dev/null 2>&1; then
  (cd dist && sha256sum "k10s-setup-$VERSION.exe" >"k10s-setup-$VERSION.exe.sha256")
else
  (cd dist && shasum -a 256 "k10s-setup-$VERSION.exe" >"k10s-setup-$VERSION.exe.sha256")
fi

echo
echo "built:"
ls -lh "$OUT" "$OUT.sha256"
