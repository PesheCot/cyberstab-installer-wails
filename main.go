//go:build !cyberstab_uninstaller && !cyberstab_manager

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	logFilePath := filepath.Join(os.TempDir(), "cyberstab-installer.log")
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logFilePath = fallbackLogPath()
		f, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}

	if err == nil {
		mw := io.MultiWriter(os.Stdout, f)
		log.SetOutput(mw)
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
		defer f.Close()
	}

	log.Printf("=========================================")
	log.Printf("Cyberstab Installer started (PID: %d)", os.Getpid())
	log.Printf("Log file: %s", logFilePath)
	log.Printf("=========================================")

	checkPlatformPrivileges()

	app := NewApp()
	cfg := &options.App{
		Title:         "Установщик Киберстаб",
		Width:         860,
		Height:        620,
		DisableResize: true,
		Frameless:     true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 251, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	}
	applyPlatformOptions(cfg)

	if runErr := wails.Run(cfg); runErr != nil {
		log.Fatal(runErr)
	}
}

func previewInstallDoneRequested() bool {
	if os.Getenv("CYBERSTAB_PREVIEW_INSTALL_DONE") == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "--preview-install-done" {
			return true
		}
	}
	return false
}
