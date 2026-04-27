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

echo "Downloading Gloss for ${os}-${arch}..."

if ! curl -fsSL "$url" -o "$tmpdir/$asset"; then
  echo
  echo "Failed to download release asset:"
  echo "  $url"
  echo
  echo "Expected asset name:"
  echo "  $asset"
  exit 1
fi

cd "$tmpdir"
unzip -q "$asset"

mkdir -p "$INSTALL_DIR"
install -m 0755 "${BIN_NAME}-${os}-${arch}" "$INSTALL_DIR/$BIN_NAME"

if [[ -t 1 ]]; then
  bold="$(printf '\033[1m')"
  green="$(printf '\033[32m')"
  yellow="$(printf '\033[33m')"
  cyan="$(printf '\033[36m')"
  reset="$(printf '\033[0m')"
else
  bold=""
  green=""
  yellow=""
  cyan=""
  reset=""
fi

echo
echo "${green}✓${reset} Gloss installed:"
echo "  ${bold}$INSTALL_DIR/$BIN_NAME${reset}"

shell_rc="$HOME/.zshrc"
if [[ "${SHELL:-}" == *"bash"* ]]; then
  shell_rc="$HOME/.bashrc"
fi

case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo
    echo "Run:"
    echo "  ${cyan}gloss version${reset}"
    ;;
  *)
    path_comment="# --- Path to Gloss ---"
    path_line="export PATH=\"$INSTALL_DIR:\$PATH\""

    echo
    echo "${yellow}!${reset} $INSTALL_DIR is not in your PATH."
    echo
    echo "Run these commands:"
    echo "  ${cyan}grep -qxF '$path_line' \"$shell_rc\" || {echo '$path_comment'; echo '$path_line'; } >>\"$shell_rc\"${reset}"
    echo "  ${cyan}source \"$shell_rc\"${reset}"
    echo
    echo "Then run:"
    echo "  ${cyan}gloss version${reset}"
    ;;
esac
