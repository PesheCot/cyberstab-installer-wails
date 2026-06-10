//go:build bindings && !cyberstab_uninstaller && !cyberstab_manager

package main

import "github.com/wailsapp/wails/v2/pkg/options"

func fallbackLogPath() string {
	return "/tmp/cyberstab-installer.log"
}

func checkPlatformPrivileges() {}

func applyPlatformOptions(cfg *options.App) {
	_ = cfg
}
