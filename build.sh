#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
rm -rf target && mkdir -p target
LDFLAGS="-X main.version=${VERSION}"

# darwin (macOS: Intel MBP / Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o target/dsh_darwin_amd64_${VERSION} .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o target/dsh_darwin_arm64_${VERSION} .

# linux (WSL 与 Linux 服务器: x86_64 / ARM64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o target/dsh_linux_amd64_${VERSION} .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o target/dsh_linux_arm64_${VERSION} .
