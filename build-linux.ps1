# Сборка Linux-установщика с Windows (через WSL).
# Wails не поддерживает прямую кросс-компиляцию Windows -> Linux.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "Building frontend (Windows)..."
Push-Location frontend
if (-not (Test-Path "node_modules")) { npm install }
npm run build
Pop-Location

function Test-WslReady {
    try {
        wsl -e bash -lc "exit 0" 2>$null | Out-Null
        return $LASTEXITCODE -eq 0
    } catch {
        return $false
    }
}

if (-not (Test-WslReady)) {
    Write-Host ""
    Write-Host "WSL не установлен или не запущен." -ForegroundColor Yellow
    Write-Host "Варианты:" -ForegroundColor Yellow
    Write-Host "  1) wsl --install -d Ubuntu  (перезагрузка, затем снова .\build-linux.ps1)" -ForegroundColor Yellow
    Write-Host "  2) GitHub Actions: Actions -> Build Linux installer -> Run workflow" -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

$winPath = (Resolve-Path $PSScriptRoot).Path
$wslPath = (wsl wslpath -a "$winPath").Trim()
if (-not $wslPath) {
    Write-Host "Cannot map path to WSL: $winPath" -ForegroundColor Red
    exit 1
}

Write-Host "Building Linux installer in WSL ($wslPath)..."
$bashCmd = "set -e; cd '$wslPath'; chmod +x build-linux.sh; ./build-linux.sh"
wsl -e bash -lc $bashCmd
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$out = Join-Path $PSScriptRoot "build\bin\install"
if (Test-Path $out) {
    Write-Host "Done: $out" -ForegroundColor Green
    Write-Host "Скопируйте install на Linux: chmod +x install && sudo ./install" -ForegroundColor Green
} else {
    Write-Host "WARNING: expected $out not found" -ForegroundColor Yellow
}
