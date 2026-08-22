#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
rm -rf target && mkdir -p target
LDFLAGS="-X main.version=${VERSION}"

# darwin
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o target/dsh_darwin_amd64_${VERSION} .

# linux
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "${LDFLAGS}" -o target/dsh_linux_386_${VERSION} .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o target/dsh_linux_amd64_${VERSION} .
CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags "${LDFLAGS}" -o target/dsh_linux_arm_${VERSION} .

# freebsd
CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags "${LDFLAGS}" -o target/dsh_freebsd_386_${VERSION} .
CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o target/dsh_freebsd_amd64_${VERSION} .
CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags "${LDFLAGS}" -o target/dsh_freebsd_arm_${VERSION} .

# windows
CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "${LDFLAGS}" -o target/dsh_windows_386_${VERSION}.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o target/dsh_windows_amd64_${VERSION}.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=arm go build -ldflags "${LDFLAGS}" -o target/dsh_windows_arm_${VERSION}.exe .