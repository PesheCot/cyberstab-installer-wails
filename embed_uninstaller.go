//go:build windows && cyberstab_uninstaller

package main

import "embed"

//go:embed frontend/dist
var assets embed.FS
