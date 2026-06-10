//go:build windows && !cyberstab_uninstaller && !cyberstab_manager

package main

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	installer "cyberstab-installer/pkg/installer"
)

func fallbackLogPath() string {
	return `C:\ProgramData\cyberstab-installer.log`
}

func checkPlatformPrivileges() {
	e := installer.NewEngine()
	if e.NeedSudo() && !previewInstallDoneRequested() {
		if installer.TryRelaunchAsAdmin(os.Args) {
			os.Exit(0)
		}
		showAdminRequiredAndExit()
	}
}

func applyPlatformOptions(cfg *options.App) {
	cfg.Windows = &windows.Options{
		DisableWindowIcon:   false,
		WebviewUserDataPath: wvDataDir(),
	}
}

func showAdminRequiredAndExit() {
	const mbIconError = 0x00000010
	const mbOkOnly = 0x00000000
	const mbTopmost = 0x00040000

	title, _ := syscall.UTF16PtrFromString("Cyberstab Installer")
	message, _ := syscall.UTF16PtrFromString(
		"Для установки Cyberstab требуются права администратора.",
	)
	user32 := syscall.NewLazyDLL("user32.dll")
	msgBoxW := user32.NewProc("MessageBoxW")
	msgBoxW.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), uintptr(mbIconError|mbOkOnly|mbTopmost))
	os.Exit(1)
}
