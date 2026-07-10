package system

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	serverPlatformWindows = "windows"
	serverPlatformLinux   = "linux"
)

// InstalledServerPlatform detects Cyberstab server bundle under installDir.
func InstalledServerPlatform(installDir string) string {
	installDir = installDirOrDefault(installDir)
	winDir := filepath.Join(installDir, "CyberstabServerWindows")
	if st, err := os.Stat(winDir); err == nil && st.IsDir() {
		return serverPlatformWindows
	}
	linuxDir := filepath.Join(installDir, "CyberstabServerLinux")
	if st, err := os.Stat(linuxDir); err == nil && st.IsDir() {
		return serverPlatformLinux
	}
	if runtime.GOOS == "windows" {
		return serverPlatformWindows
	}
	return serverPlatformLinux
}

// ServerUpdateArchiveName returns the Nextcloud archive name for the installed server OS.
func ServerUpdateArchiveName(installDir string) string {
	if InstalledServerPlatform(installDir) == serverPlatformLinux {
		return "CyberstabServerLinux.zip"
	}
	return "CyberstabServerWindows.zip"
}
