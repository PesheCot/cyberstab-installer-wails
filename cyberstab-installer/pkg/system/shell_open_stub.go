//go:build !windows

package system

import "fmt"

func RevealPathInExplorer(path string) error {
	return fmt.Errorf("открытие проводника поддерживается только в Windows")
}
