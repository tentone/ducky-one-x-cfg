#!/usr/bin/env sh
set -eu

mkdir -p dist
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o dist/ducky-config ./cmd/ducky-config
echo "Built dist/ducky-config"
