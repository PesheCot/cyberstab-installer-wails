//go:build clionly && linux

package main

import (
	"log"

	installer "cyberstab-installer/pkg/installer"
)

func checkPlatformPrivileges() {
	e := installer.NewEngine()
	if e.NeedSudo() {
		log.Fatal("Для установки Cyberstab запустите установщик от root: sudo ./install-linux -c")
	}
}

func checkUninstallPrivileges() {
	e := installer.NewEngine()
	if e.NeedSudo() {
		log.Fatal("Для удаления Cyberstab запустите от root: sudo ./cyberstab-uninstaller-linux -c")
	}
}
