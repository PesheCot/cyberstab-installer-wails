//go:build linux && !cyberstab_uninstaller

package main

// Embedded Linux uninstaller is optional until build-linux.sh produces it.
var uninstallerData []byte
