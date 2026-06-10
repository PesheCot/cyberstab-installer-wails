//go:build linux && cyberstab_uninstaller

package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	installer "cyberstab-installer/pkg/installer"
)

func main() {
	e := installer.NewEngine()
	if e.NeedSudo() {
		log.Fatal("Для удаления Cyberstab запустите деинсталлятор от root: sudo ./cyberstab-uninstaller")
	}

	app := NewApp()

	err := wails.Run(&options.App{
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
	})
	if err != nil {
		log.Fatal(err)
	}
}
