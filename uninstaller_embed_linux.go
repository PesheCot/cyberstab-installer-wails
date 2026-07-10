//go:build linux && !cyberstab_uninstaller

package main

import _ "embed"

//go:embed uninstaller/cyberstab-uninstaller
var uninstallerData []byte
