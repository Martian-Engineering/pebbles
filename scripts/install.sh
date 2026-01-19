#!/usr/bin/env sh
set -eu

REPO="Martian-Engineering/pebbles"
VERSION="${PB_VERSION:-latest}"
INSTALL_DIR="${PB_INSTALL_DIR:-$HOME/.local/bin}"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
  echo "tar is required" >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin|linux) ;; 
  *)
    echo "unsupported OS: $OS" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;; 
  arm64|aarch64) ARCH="arm64" ;; 
  *)
    echo "unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/pb-${OS}-${ARCH}.tar.gz"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/pb-${OS}-${ARCH}.tar.gz"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p "$INSTALL_DIR"

curl -fsSL "$URL" -o "$TMP_DIR/pb.tar.gz"
tar -xzf "$TMP_DIR/pb.tar.gz" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/pb" ]; then
  echo "pb binary not found in archive" >&2
  exit 1
fi

mv "$TMP_DIR/pb" "$INSTALL_DIR/pb"
chmod 755 "$INSTALL_DIR/pb"

echo "Installed pb to $INSTALL_DIR/pb"
