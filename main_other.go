//go:build !windows && !linux && !cyberstab_uninstaller && !cyberstab_manager

package main

import "github.com/wailsapp/wails/v2/pkg/options"

func fallbackLogPath() string {
	return ""
}

func checkPlatformPrivileges() {}

func applyPlatformOptions(cfg *options.App) {
	_ = cfg
}
