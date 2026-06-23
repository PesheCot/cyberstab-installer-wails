//go:build cyberstab_uninstaller && !clionly

package main

import "embed"

//go:embed frontend/dist
var assets embed.FS
