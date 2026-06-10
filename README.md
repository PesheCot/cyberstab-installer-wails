# Cyberstab Installer (Wails UI)

## Требования
- Go
- Node.js (для сборки UI)
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Запуск в dev-режиме
Из папки `cyberstab-installer-wails`:

1) Установить зависимости фронтенда:

```powershell
cd frontend
npm install
cd ..
```

2) Запустить:

```powershell
wails dev
```

### Если `wails dev` зависает на `Compiling frontend`
Иногда Wails ждёт Vite dev server (не успевает обнаружить или не показывает вывод). Тогда запускайте Vite отдельно, а Wails — с флагами:

Окно 1 (Vite):

```powershell
cd frontend
npm run dev -- --host 127.0.0.1 --port 5173
```

Окно 2 (Wails):

```powershell
cd ..\cyberstab-installer-wails
wails dev -s -frontenddevserverurl http://127.0.0.1:5173 -v 2 -nocolour
```

## Сборка релиза (Windows)
```powershell
.\build-windows.ps1
```

## Сборка Linux-установщика с Windows

Wails **не умеет** собирать Linux `.exe`/бинарник напрямую с Windows (`Crosscompiling to Linux not currently supported`).
Нужен **WSL** (Ubuntu) или **GitHub Actions**.

### Вариант A: WSL на этой машине

1. Установите WSL: `wsl --install -d Ubuntu` (перезагрузка)
2. В Ubuntu:
   ```bash
   sudo apt update
   sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev build-essential
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
3. На Windows из папки проекта:
   ```powershell
   .\build-linux.ps1
   ```
4. Результат: `build\bin\install` — скопируйте на Linux и запустите:
   ```bash
   chmod +x install
   sudo ./install          # GUI, если есть DISPLAY/WAYLAND_DISPLAY
   sudo ./install -с       # консольный мастер (кириллическая «с»)
   sudo ./install -c       # то же: алиас --console
   ```

   Без графической сессии (SSH без X11) установщик автоматически запускает консольный мастер.

### Вариант B: GitHub Actions (без WSL)

1. Запушьте код в GitHub
2. Откройте **Actions → Build Linux installer → Run workflow**
3. Скачайте артефакт `cyberstab-install-linux-amd64`

### Сборка на самом Linux
```bash
chmod +x build-linux.sh
./build-linux.sh
```

Скрипт собирает `build/bin/install` и `build/bin/cyberstab-uninstaller`.

### Консольный режим (install / uninstall)

| Команда | Поведение |
|---------|-----------|
| `sudo ./install` | GUI на рабочем столе; CLI по SSH без DISPLAY |
| `sudo ./install -с` | Всегда консольный мастер установки |
| `sudo ./cyberstab-uninstaller` | GUI удаления (если есть графическая сессия) |
| `sudo ./cyberstab-uninstaller -с` | Консольное удаление |

Ключи: `-с` (кириллица), `-c`, `--console`.

На Windows: `install.exe -с` — консоль; без ключа — GUI.

### Чтобы на Windows не появлялась консоль
В проекте уже включено `-H windowsgui` в `wails.json`, поэтому релизный `.exe` будет запускаться **только окном**.
Если вы собираете вручную, добавляйте:

```powershell
wails build -ldflags "-H windowsgui"
```

## Примечание
Backend использует существующий движок из `../cyberstab-installer/pkg/...`.

