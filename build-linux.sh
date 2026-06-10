#!/usr/bin/env bash
# Сборка Linux-установщика (запускать в Linux или WSL).
# Зависимости (Ubuntu/Debian): libgtk-3-dev libwebkit2gtk-4.0-dev build-essential
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "==> Frontend"
pushd frontend >/dev/null
if [[ ! -d node_modules ]]; then npm install; fi
npm run build
popd >/dev/null

if ! command -v wails >/dev/null 2>&1; then
  echo "Wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  exit 1
fi

echo "==> Linux installer"
wails build -platform linux/amd64 -o install -ldflags ""

echo "==> Linux uninstaller"
wails build -platform linux/amd64 -tags "cyberstab_uninstaller" -o cyberstab-uninstaller -ldflags ""

echo "==> Done:"
echo "    $ROOT/build/bin/install"
echo "    $ROOT/build/bin/cyberstab-uninstaller"
