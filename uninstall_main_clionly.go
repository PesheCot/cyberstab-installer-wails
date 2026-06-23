//go:build clionly && cyberstab_uninstaller

package main

import (
	"log"
)

func main() {
	setupLogging()
	checkUninstallPrivileges()
	app := NewApp()
	if err := runCLIUninstall(app); err != nil {
		log.Fatal(err)
	}
}
