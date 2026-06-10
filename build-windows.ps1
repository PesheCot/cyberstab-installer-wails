# Сборка GUI-деинсталлятора и установщика Wails (Windows).
# 1) Собирается cyberstab-uninstaller.exe (Wails, тот же UI-стиль) — он встраивается в установщик через //go:embed.
# 2) Собирается install.exe.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "Generating Windows app icon..."
$iconScript = @'
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

root = Path.cwd()
out = root / "build" / "appicon.png"
out.parent.mkdir(parents=True, exist_ok=True)

size = 1024
scale = size / 38.0
img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
draw = ImageDraw.Draw(img)

def pts(values):
    return [(int(x * scale), int(y * scale)) for x, y in values]

blue = (52, 87, 231, 255)
cyan = (68, 212, 247, 255)
black = (0, 0, 0, 255)
white = (255, 255, 255, 255)

# White shield body with transparent outside, close to the Figma header logo.
shield = pts([(6.8, 5.8), (31.2, 5.8), (31.2, 28.9), (19.0, 34.0), (6.8, 28.9)])
draw.polygon(shield, fill=white)

# Top and bottom Cyberstab strokes.
draw.rectangle([int(6.9 * scale), int(6.5 * scale), int(31.1 * scale), int(9.5 * scale)], fill=blue)
bottom = pts([(6.9, 24.1), (9.9, 24.1), (9.9, 27.2), (19.0, 30.8), (28.1, 27.2), (28.1, 24.1), (31.1, 24.1), (31.1, 29.0), (19.0, 34.0), (6.9, 29.0)])
draw.polygon(bottom, fill=blue)

# Letter K.
try:
    font = ImageFont.truetype("arialbd.ttf", int(16 * scale))
except Exception:
    font = ImageFont.load_default()
draw.text((int(14.0 * scale), int(10.1 * scale)), "K", fill=black, font=font)

img.save(out)
print(out)
'@
$iconScriptPath = Join-Path $PSScriptRoot "build\generate_appicon.py"
New-Item -ItemType Directory -Force -Path (Split-Path $iconScriptPath) | Out-Null
Set-Content -Path $iconScriptPath -Value $iconScript -Encoding UTF8
python $iconScriptPath
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$windowsIcon = Join-Path $PSScriptRoot "build\windows\icon.ico"
if (Test-Path $windowsIcon) {
  Remove-Item -Force $windowsIcon
}

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
wails build -o install.exe
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Done."
