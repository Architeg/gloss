#!/usr/bin/env bash
set -euo pipefail

REPO="Architeg/gloss"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
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

if ! curl -fL "$url" -o "$tmpdir/$asset"; then
  echo
  echo "Failed to download release asset:"
  echo "  $url"
  echo
  echo "Expected asset name:"
  echo "  $asset"
  echo
  echo "Check that the GitHub release is published and contains this exact file."
  exit 1
fi

cd "$tmpdir"
unzip -q "$asset"

mkdir -p "$INSTALL_DIR"
install -m 0755 "${BIN_NAME}-${os}-${arch}" "$INSTALL_DIR/$BIN_NAME"

echo
echo "Installed to $INSTALL_DIR/$BIN_NAME"

case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "Run: gloss version"
    ;;
  *)
    echo
    echo "Note: $INSTALL_DIR is not in your PATH."
    echo "Add this to your shell config:"
    echo
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo
    echo "Then reload your shell and run:"
    echo
    echo "  gloss version"
    ;;
esac
