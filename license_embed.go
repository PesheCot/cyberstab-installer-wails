//go:build !cyberstab_uninstaller && !cyberstab_manager && (!bindings || clionly)

package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed assets/user_agreement.txt
var userAgreementText string

// materializeUserAgreementFile writes the embedded license text to a temp file
// and returns its path for CLI users to open/read.
func materializeUserAgreementFile() (string, error) {
	dir := filepath.Join(os.TempDir(), "cyberstab-installer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "user_agreement.txt")
	if err := os.WriteFile(path, []byte(userAgreementText), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
