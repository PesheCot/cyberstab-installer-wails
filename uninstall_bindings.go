//go:build cyberstab_uninstaller && bindings

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func checkUninstallPrivileges() {}

func applyPlatformOptions(cfg *options.App) {
	_ = cfg
}

// Minimal main for Wails bindings generation when building the uninstaller.
func main() {
	setupLogging()
	app := NewApp()
	cfg := &options.App{
		Title:  "Киберстаб — удаление",
		Width:  860,
		Height: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind:      []interface{}{app},
	}
	applyPlatformOptions(cfg)
	if runErr := wails.Run(cfg); runErr != nil {
		log.Fatal(runErr)
	}
	_ = filepath.Join(os.TempDir(), "cyberstab-uninstaller.log")
}
