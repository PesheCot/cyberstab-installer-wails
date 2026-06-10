//go:build linux && cyberstab_uninstaller

package main

func (a *App) GetUIMode() string {
	return "uninstall"
}
