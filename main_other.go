//go:build !windows && !linux && !cyberstab_uninstaller && !cyberstab_manager && !bindings

package main

import "github.com/wailsapp/wails/v2/pkg/options"

func checkPlatformPrivileges() {}

func applyPlatformOptions(cfg *options.App) {
	_ = cfg
}
