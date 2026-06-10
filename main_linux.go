//go:build linux && !cyberstab_uninstaller && !cyberstab_manager

package main

import (
	"log"

	"github.com/wailsapp/wails/v2/pkg/options"

	installer "cyberstab-installer/pkg/installer"
)

func fallbackLogPath() string {
	return "/tmp/cyberstab-installer.log"
}

func checkPlatformPrivileges() {
	e := installer.NewEngine()
	if e.NeedSudo() && !previewInstallDoneRequested() {
		log.Fatal("Для установки Cyberstab запустите установщик от root: sudo ./install")
	}
}

func applyPlatformOptions(cfg *options.App) {
	_ = cfg
}
