//go:build linux && !cyberstab_uninstaller

package main

import _ "embed"

//go:embed uninstaller/linux-uninstaller.bin
var uninstallerData []byte
