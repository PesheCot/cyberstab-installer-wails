package main

import (
	"os"
	"runtime"
	"strings"
)

const consoleFlagCyrillic = "-с" // U+0441

// parseLaunchMode returns true when the user requested console mode (-с, -c, --console).
func parseLaunchMode(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case consoleFlagCyrillic, "-c", "--console":
			return true
		}
	}
	return false
}

func hasGraphicalSession() bool {
	if runtime.GOOS == "linux" {
		return strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
			strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
	}
	return true
}

func shouldRunConsole(force bool) bool {
	if previewInstallDoneRequested() {
		return false
	}
	return force || !hasGraphicalSession()
}

func previewInstallDoneRequested() bool {
	if os.Getenv("CYBERSTAB_PREVIEW_INSTALL_DONE") == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "--preview-install-done" {
			return true
		}
	}
	return false
}
