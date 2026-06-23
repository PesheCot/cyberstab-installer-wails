//go:build clionly && !cyberstab_uninstaller && !cyberstab_manager

package main

import (
	"log"
)

func main() {
	setupLogging()
	checkPlatformPrivileges()
	app := NewApp()
	if err := runCLIInstall(app); err != nil {
		log.Fatal(err)
	}
}
