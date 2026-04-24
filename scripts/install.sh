#!/usr/bin/env bash
set -euo pipefail

REPO="Architeg/gloss"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BIN_NAME="gloss"

uname_s="$(uname -s)"
uname_m="$(uname -m)"

case "$uname_s" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "Unsupported OS: $uname_s"
    exit 1
    ;;
esac

case "$uname_m" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64) arch="amd64" ;;
  *)
    echo "Unsupported architecture: $uname_m"
    exit 1
    ;;
esac

asset="${BIN_NAME}-${os}-${arch}.zip"

if [[ "$VERSION" == "latest" ]]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "Downloading $url"
curl -fsSL "$url" -o "$tmpdir/$asset"

cd "$tmpdir"
unzip -q "$asset"

mkdir -p "$INSTALL_DIR"
install -m 0755 "${BIN_NAME}-${os}-${arch}" "$INSTALL_DIR/$BIN_NAME"

echo
echo "Installed to $INSTALL_DIR/$BIN_NAME"
echo "Run: gloss version"
