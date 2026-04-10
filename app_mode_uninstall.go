//go:build windows && cyberstab_uninstaller

package main

// GetUIMode tells the frontend which root view to show (uninstaller build).
func (a *App) GetUIMode() string {
	return "uninstall"
}
