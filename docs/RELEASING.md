# Releasing Pebbles

Pebbles releases are built with version metadata injected at link time.

## Version Metadata

The binary exposes build metadata via `pb --version` or `pb version`:

- `buildVersion`: release tag (e.g., `v0.3.1`)
- `buildCommit`: short git SHA
- `buildDate`: UTC date (YYYY-MM-DD)

These are set via Go linker flags:

```bash
-ldflags "-X main.buildVersion=v0.3.1 -X main.buildCommit=<sha> -X main.buildDate=2026-01-20"
```

## Release Script

Use the scripted flow from the repo root:

```bash
scripts/release.sh v0.3.1
```

The script:

- builds darwin/linux × amd64/arm64 binaries with metadata
- packages `pb-<os>-<arch>.tar.gz`
- creates a GitHub release and uploads assets

## Manual Steps (if needed)

```bash
VERSION=v0.3.1
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%d)
LD_FLAGS="-X main.buildVersion=${VERSION} -X main.buildCommit=${COMMIT} -X main.buildDate=${DATE}"

GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$LD_FLAGS" -o pb ./cmd/pb
```
