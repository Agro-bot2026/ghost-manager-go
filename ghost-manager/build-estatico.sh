#!/bin/bash
# build-estatico.sh — compila el ghost-manager en Go ESTÁTICO (sin cgo) para el VPS
set -e
cd /root/ghost-go/gm
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
gofmt -l -w *.go >/dev/null
GOFLAGS=-mod=mod go build -trimpath -ldflags "-s -w" -o /tmp/ghost-manager-go-static .
echo "BUILD ESTÁTICO OK"
ls -la /tmp/ghost-manager-go-static
file /tmp/ghost-manager-go-static 2>/dev/null || true
