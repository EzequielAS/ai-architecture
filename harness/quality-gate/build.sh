#!/usr/bin/env bash
# Compila o quality-gate (Go puro, sem CGO) em bin/.
#   ./build.sh        → binário para a máquina atual
#   ./build.sh all    → binários para Linux, macOS e Windows (amd64 e arm64)
set -euo pipefail

cd "$(dirname "$0")"
export CGO_ENABLED=0
mkdir -p bin

build() { # <goos> <goarch> <nome>
  echo "→ bin/$3"
  GOOS="$1" GOARCH="$2" go build -trimpath -ldflags="-s -w" -o "bin/$3" .
}

if [[ "${1:-}" == "all" ]]; then
  build linux   amd64 quality-gate-linux-amd64
  build linux   arm64 quality-gate-linux-arm64
  build darwin  amd64 quality-gate-darwin-amd64
  build darwin  arm64 quality-gate-darwin-arm64
  build windows amd64 quality-gate-windows-amd64.exe
  build windows arm64 quality-gate-windows-arm64.exe
else
  go build -trimpath -ldflags="-s -w" -o bin/quality-gate .
  echo "→ bin/quality-gate"
fi
