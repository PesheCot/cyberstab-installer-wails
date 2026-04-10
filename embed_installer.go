//go:build !cyberstab_uninstaller

package main

import "embed"

//go:embed frontend/dist
var assets embed.FS

//go:embed uninstaller/cyberstab-uninstaller.exe
var uninstallerData []byte
