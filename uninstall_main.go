//go:build windows && cyberstab_uninstaller

package main

import (
	"log"
	"os"
	stdruntime "runtime"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	installer "cyberstab-installer/pkg/installer"
)

func showUninstallAdminRequiredAndExit() {
	const mbIconError = 0x00000010
	const mbOkOnly = 0x00000000
	const mbTopmost = 0x00040000

	title, _ := syscall.UTF16PtrFromString("Cyberstab Uninstaller")
	message, _ := syscall.UTF16PtrFromString(
		"Для удаления Cyberstab требуются права администратора.",
	)
	user32 := syscall.NewLazyDLL("user32.dll")
	msgBoxW := user32.NewProc("MessageBoxW")
	msgBoxW.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), uintptr(mbIconError|mbOkOnly|mbTopmost))
	os.Exit(1)
}

func main() {
	if stdruntime.GOOS == "windows" {
		e := installer.NewEngine()
		if e.NeedSudo() {
			if installer.TryRelaunchAsAdmin(os.Args) {
				os.Exit(0)
			}
			showUninstallAdminRequiredAndExit()
		}
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
		Windows: &windows.Options{
			DisableWindowIcon:   false,
			WebviewUserDataPath: wvDataDir(),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
