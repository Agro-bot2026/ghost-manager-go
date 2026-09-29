#!/bin/bash
# build.sh — compila el ghost-manager en Go y hace un smoke test rápido
set -e
cd /root/ghost-go/gm
gofmt -l -w *.go >/dev/null
echo "--- vet ---"
GOFLAGS=-mod=mod go vet ./... || true
echo "--- build ---"
GOFLAGS=-mod=mod go build -o /tmp/ghost-manager-go .
echo "BUILD OK → /tmp/ghost-manager-go"
echo "--- smoke ---"
BIN=/tmp/ghost-manager-go
$BIN --version
$BIN --firma | sed 's/^/   /'
