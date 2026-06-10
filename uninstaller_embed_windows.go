//go:build windows && !cyberstab_uninstaller

package main

import _ "embed"

//go:embed uninstaller/cyberstab-uninstaller.exe
var uninstallerData []byte
