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

resolve_wails_binary() {
  local out="$1"
  if [[ -f "$out" ]]; then
    printf '%s\n' "$out"
    return 0
  fi
  if [[ -d "$out" ]]; then
    if [[ -f "$out/$(basename "$out")" ]]; then
      printf '%s\n' "$out/$(basename "$out")"
      return 0
    fi
    local found
    found="$(find "$out" -maxdepth 2 -type f -perm -111 | head -n 1)"
    if [[ -n "$found" ]]; then
      printf '%s\n' "$found"
      return 0
    fi
  fi
  return 1
}

echo "==> Linux uninstaller (embed into installer)"
wails build -platform linux/amd64 -tags "cyberstab_uninstaller" -o cyberstab-uninstaller -ldflags ""

built_out="$ROOT/build/bin/cyberstab-uninstaller"
built="$(resolve_wails_binary "$built_out" || true)"
if [[ -z "${built:-}" || ! -f "$built" ]]; then
  echo "ERROR: expected uninstaller binary under $built_out"
  ls -la "$ROOT/build/bin" || true
  exit 1
fi

target_dir="$ROOT/uninstaller"
target="$target_dir/linux-uninstaller.bin"
mkdir -p "$target_dir"
cp -f "$built" "$target"

echo "==> Linux installer"
wails build -platform linux/amd64 -o install -ldflags ""

echo "==> Done:"
echo "    $ROOT/build/bin/install"
echo "    $ROOT/build/bin/cyberstab-uninstaller"
