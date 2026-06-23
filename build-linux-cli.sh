#!/usr/bin/env bash
# Статическая сборка консольного установщика (без GTK/Wails).
# Работает на Astra 1.7 (glibc 2.28) и других старых дистрибутивах.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

mkdir -p build/bin
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

echo "==> Linux CLI installer (static, any distro)"
go build -tags "clionly" -ldflags "-s -w" -o build/bin/install-linux-static .

echo "==> Linux CLI uninstaller (static)"
go build -tags "cyberstab_uninstaller,clionly" -ldflags "-s -w" -o build/bin/cyberstab-uninstaller-static .

echo "==> Done:"
echo "    $ROOT/build/bin/install-linux-static"
echo "    $ROOT/build/bin/cyberstab-uninstaller-static"
