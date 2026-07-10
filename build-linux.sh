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

echo "==> Linux uninstaller (embed into installer)"
wails build -platform linux/amd64 -tags "cyberstab_uninstaller" -o cyberstab-uninstaller -ldflags ""

# wails кладёт результат в build/bin — копируем в uninstaller/ для //go:embed
built="$ROOT/build/bin/cyberstab-uninstaller"
target_dir="$ROOT/uninstaller"
target="$target_dir/cyberstab-uninstaller"
if [[ ! -f "$built" ]]; then
  echo "ERROR: expected $built not found; cannot embed uninstaller."
  exit 1
fi
mkdir -p "$target_dir"
cp -f "$built" "$target"

echo "==> Linux installer"
wails build -platform linux/amd64 -o install -ldflags ""

echo "==> Done:"
echo "    $ROOT/build/bin/install"
echo "    $ROOT/build/bin/cyberstab-uninstaller"
