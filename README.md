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

## Сборка релиза
```powershell
wails build
```

### Чтобы на Windows не появлялась консоль
В проекте уже включено `-H windowsgui` в `wails.json`, поэтому релизный `.exe` будет запускаться **только окном**.
Если вы собираете вручную, добавляйте:

```powershell
wails build -ldflags "-H windowsgui"
```

## Примечание
Backend использует существующий движок из `../cyberstab-installer/pkg/...`.

