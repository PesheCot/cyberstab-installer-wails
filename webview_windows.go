//go:build windows

package main

import (
	"os"
	"path/filepath"
	stdruntime "runtime"
)

// wvDataDir returns an isolated path for WebView2 user data.
func wvDataDir() string {
	if stdruntime.GOOS != "windows" {
		return ""
	}
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	p := filepath.Join(base, "CyberstabInstaller", "WebView2")
	_ = os.MkdirAll(p, 0755)
	return p
}
