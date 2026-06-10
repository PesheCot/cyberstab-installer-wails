//go:build linux && cyberstab_uninstaller && !bindings

package main

import (
	"log"
	"os"
)

func main() {
	setupLogging()
	checkUninstallPrivileges()

	forceConsole := parseLaunchMode(os.Args[1:])
	if shouldRunConsole(forceConsole) {
		app := NewApp()
		if err := runCLIUninstall(app); err != nil {
			log.Fatal(err)
		}
		return
	}
	runGUIUninstall()
}
