# Install

## Curl Installer

The preferred install flow is a curl-based script that downloads a prebuilt
binary from GitHub releases.

```bash
curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/main/scripts/install.sh | sh
```

Optional environment variables:

- `PB_VERSION`: release tag to install (default: `latest`)
- `PB_INSTALL_DIR`: destination directory (default: `~/.local/bin`)

Examples:

```bash
PB_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/main/scripts/install.sh | sh
PB_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/main/scripts/install.sh | sh
```

## Release Asset Naming

The installer expects release artifacts named as follows:

```
pb-<os>-<arch>.tar.gz
```

Supported values:

- `os`: `darwin`, `linux`
- `arch`: `amd64`, `arm64`

Each archive should contain a single `pb` binary at the root.

## Local Build

```bash
go build -o pb ./cmd/pb
```
