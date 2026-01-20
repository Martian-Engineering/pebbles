#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: scripts/release.sh <version>

Builds release artifacts and creates a GitHub release.

Example:
  scripts/release.sh v0.3.1
USAGE
}

if [ "$#" -ne 1 ]; then
  usage
  exit 1
fi

VERSION="$1"
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must look like vX.Y.Z" >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is required" >&2
  exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
  echo "tar is required" >&2
  exit 1
fi

COMMIT="$(git rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%d)"
LD_FLAGS="-X main.buildVersion=${VERSION} -X main.buildCommit=${COMMIT} -X main.buildDate=${DATE}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

for OS in darwin linux; do
  for ARCH in amd64 arm64; do
    OUT_DIR="$TMP_DIR/pb-${OS}-${ARCH}"
    mkdir -p "$OUT_DIR"
    GOOS="$OS" GOARCH="$ARCH" CGO_ENABLED=0 go build -ldflags "$LD_FLAGS" -o "$OUT_DIR/pb" ./cmd/pb
    tar -czf "$TMP_DIR/pb-${OS}-${ARCH}.tar.gz" -C "$OUT_DIR" pb
  done
done

gh release create "$VERSION" "$TMP_DIR"/pb-*.tar.gz --title "$VERSION" --notes "Release $VERSION" --target master

echo "Release $VERSION created."
