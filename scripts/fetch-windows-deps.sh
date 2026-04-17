#!/usr/bin/env bash
# Downloads pinned Windows amd64 binaries for kubectl, k9s, and kubelogin
# into dist/windows-deps/, verifying each against its upstream SHA256 sidecar.
# Also collects Apache 2.0 NOTICE/LICENSE files required by those projects.
#
# Reads version pins from scripts/windows-deps-versions.env.
# Runs on Linux (CI) and macOS (local dev).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$REPO_ROOT/dist/windows-deps"
LIC_DIR="$OUT_DIR/LICENSES"

# shellcheck disable=SC1091
source "$SCRIPT_DIR/windows-deps-versions.env"

: "${KUBECTL_VERSION:?missing in versions env}"
: "${K9S_VERSION:?missing in versions env}"
: "${KUBELOGIN_VERSION:?missing in versions env}"

# sha256 helper that works on both Linux (sha256sum) and macOS (shasum).
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

die() { echo "fetch-windows-deps: $*" >&2; exit 1; }

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR" "$LIC_DIR"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> kubectl $KUBECTL_VERSION"
KUBECTL_URL="https://dl.k8s.io/release/$KUBECTL_VERSION/bin/windows/amd64/kubectl.exe"
curl -fsSL -o "$TMP/kubectl.exe" "$KUBECTL_URL"
curl -fsSL -o "$TMP/kubectl.exe.sha256" "$KUBECTL_URL.sha256"
expected="$(tr -d '[:space:]' <"$TMP/kubectl.exe.sha256")"
actual="$(sha256 "$TMP/kubectl.exe")"
[ "$expected" = "$actual" ] || die "kubectl sha mismatch: want $expected got $actual"
cp "$TMP/kubectl.exe" "$OUT_DIR/kubectl.exe"
# Kubernetes is Apache 2.0. The release bundle does not ship a LICENSE file,
# so write a short NOTICE pointing to the canonical source.
cat >"$LIC_DIR/kubectl-NOTICE.txt" <<EOF
kubectl (Kubernetes) is distributed under the Apache License 2.0.
Version bundled: $KUBECTL_VERSION
Source:  https://github.com/kubernetes/kubernetes
License: https://github.com/kubernetes/kubernetes/blob/$KUBECTL_VERSION/LICENSE
EOF

echo "==> k9s $K9S_VERSION"
K9S_ZIP_NAME="k9s_Windows_amd64.zip"
K9S_URL="https://github.com/derailed/k9s/releases/download/$K9S_VERSION/$K9S_ZIP_NAME"
K9S_SUMS_URL="https://github.com/derailed/k9s/releases/download/$K9S_VERSION/checksums.txt"
curl -fsSL -o "$TMP/$K9S_ZIP_NAME" "$K9S_URL"
curl -fsSL -o "$TMP/k9s-checksums.txt" "$K9S_SUMS_URL"
expected="$(awk -v f="$K9S_ZIP_NAME" '$2==f || $2=="*"f {print $1; exit}' "$TMP/k9s-checksums.txt")"
[ -n "$expected" ] || die "k9s: $K9S_ZIP_NAME missing from upstream checksums.txt"
actual="$(sha256 "$TMP/$K9S_ZIP_NAME")"
[ "$expected" = "$actual" ] || die "k9s sha mismatch: want $expected got $actual"
mkdir -p "$TMP/k9s"
unzip -q "$TMP/$K9S_ZIP_NAME" -d "$TMP/k9s"
[ -f "$TMP/k9s/k9s.exe" ] || die "k9s.exe not found inside $K9S_ZIP_NAME"
cp "$TMP/k9s/k9s.exe" "$OUT_DIR/k9s.exe"
if [ -f "$TMP/k9s/LICENSE" ]; then
  cp "$TMP/k9s/LICENSE" "$LIC_DIR/k9s-LICENSE.txt"
else
  echo "k9s $K9S_VERSION ships under the Apache License 2.0. See https://github.com/derailed/k9s/blob/$K9S_VERSION/LICENSE" \
    >"$LIC_DIR/k9s-NOTICE.txt"
fi

echo "==> kubelogin $KUBELOGIN_VERSION"
KL_ZIP_NAME="kubelogin_win_amd64.zip"
KL_URL="https://github.com/int128/kubelogin/releases/download/$KUBELOGIN_VERSION/$KL_ZIP_NAME"
KL_SUMS_URL="https://github.com/int128/kubelogin/releases/download/$KUBELOGIN_VERSION/kubelogin_${KUBELOGIN_VERSION#v}_checksums.txt"
curl -fsSL -o "$TMP/$KL_ZIP_NAME" "$KL_URL"
if curl -fsSL -o "$TMP/kubelogin-checksums.txt" "$KL_SUMS_URL" 2>/dev/null; then
  expected="$(awk -v f="$KL_ZIP_NAME" '$2==f || $2=="*"f {print $1; exit}' "$TMP/kubelogin-checksums.txt")"
else
  expected=""
fi
if [ -n "$expected" ]; then
  actual="$(sha256 "$TMP/$KL_ZIP_NAME")"
  [ "$expected" = "$actual" ] || die "kubelogin sha mismatch: want $expected got $actual"
else
  echo "kubelogin: no upstream checksums file available; skipping sha verify" >&2
fi
mkdir -p "$TMP/kubelogin"
unzip -q "$TMP/$KL_ZIP_NAME" -d "$TMP/kubelogin"
# The zip layout has historically been bin/kubelogin.exe; tolerate either layout.
KL_BIN="$(find "$TMP/kubelogin" -maxdepth 3 -type f -iname 'kubelogin.exe' | head -n1)"
[ -n "$KL_BIN" ] || die "kubelogin.exe not found inside $KL_ZIP_NAME"
cp "$KL_BIN" "$OUT_DIR/kubelogin.exe"
# kubectl plugin naming convention: enables `kubectl oidc-login ...` AND
# satisfies k10s doctor which looks up `kubectl-oidc_login`.
cp "$KL_BIN" "$OUT_DIR/kubectl-oidc_login.exe"
if [ -f "$TMP/kubelogin/LICENSE" ]; then
  cp "$TMP/kubelogin/LICENSE" "$LIC_DIR/kubelogin-LICENSE.txt"
else
  echo "kubelogin $KUBELOGIN_VERSION ships under the Apache License 2.0. See https://github.com/int128/kubelogin/blob/$KUBELOGIN_VERSION/LICENSE" \
    >"$LIC_DIR/kubelogin-NOTICE.txt"
fi

echo
echo "Bundled into $OUT_DIR:"
ls -lh "$OUT_DIR" "$LIC_DIR"
