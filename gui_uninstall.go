//go:build cyberstab_uninstaller && !bindings && !clionly

package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func runGUIUninstall() {
	app := NewApp()
	cfg := &options.App{
		Title:         "Киберстаб — удаление",
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
	if err := wails.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
