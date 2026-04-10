# Сборка GUI-деинсталлятора и установщика Wails (Windows).
# 1) Собирается cyberstab-uninstaller.exe (Wails, тот же UI-стиль) — он встраивается в установщик через //go:embed.
# 2) Собирается cyberstab-installer.exe.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "npm install / build frontend..."
Push-Location frontend
if (-not (Test-Path "node_modules")) { npm install }
npm run build
Pop-Location

Write-Host "Building cyberstab-uninstaller.exe (Wails, tags cyberstab_uninstaller)..."
wails build -tags "cyberstab_uninstaller" -o cyberstab-uninstaller.exe
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# wails кладёт результат в build/bin — копируем в uninstaller/ для //go:embed
$built = Join-Path $PSScriptRoot "build\bin\cyberstab-uninstaller.exe"
if (Test-Path $built) {
  $targetDir = Join-Path $PSScriptRoot "uninstaller"
  New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
  Copy-Item -Force $built (Join-Path $targetDir "cyberstab-uninstaller.exe")
} else {
  Write-Host "WARNING: expected $built not found; embed may fail."
}

Write-Host "Building cyberstab-installer (wails)..."
wails build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Done."
