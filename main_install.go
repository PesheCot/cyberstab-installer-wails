//go:build !cyberstab_uninstaller && !cyberstab_manager && !bindings && !clionly

package main

import (
	"log"
	"os"
)

func main() {
	setupLogging()
	checkPlatformPrivileges()

	forceConsole := parseLaunchMode(os.Args[1:])
	if shouldRunConsole(forceConsole) {
		app := NewApp()
		if err := runCLIInstall(app); err != nil {
			log.Fatal(err)
		}
		return
	}
	runGUIInstall()
}
