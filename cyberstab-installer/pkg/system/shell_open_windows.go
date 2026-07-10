//go:build windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RevealPathInExplorer opens Windows Explorer and selects the given file or folder.
func RevealPathInExplorer(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("пустой путь")
	}
	path = filepath.Clean(path)
	if _, err := os.Stat(path); err != nil {
		return err
	}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return exec.Command("explorer", path).Start()
	}
	return exec.Command("explorer", "/select,", path).Start()
}
