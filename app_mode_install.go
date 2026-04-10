//go:build !cyberstab_uninstaller && !cyberstab_manager

package main

// GetUIMode tells the frontend which root view to show (installer build).
func (a *App) GetUIMode() string {
	return "install"
}
