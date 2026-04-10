//go:build cyberstab_manager

package main

// GetUIMode tells the frontend which root view to show (manager build).
func (a *App) GetUIMode() string {
	return "manager"
}

