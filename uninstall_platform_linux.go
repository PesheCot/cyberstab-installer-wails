//go:build linux && cyberstab_uninstaller && !bindings && !clionly

package main

import (
	"log"

	"github.com/wailsapp/wails/v2/pkg/options"

	installer "cyberstab-installer/pkg/installer"
)

func checkUninstallPrivileges() {
	e := installer.NewEngine()
	if e.NeedSudo() {
		log.Fatal("Для удаления Cyberstab запустите деинсталлятор от root: sudo ./cyberstab-uninstaller")
	}
}

func applyPlatformOptions(cfg *options.App) {
	_ = cfg
}
