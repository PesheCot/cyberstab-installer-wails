//go:build !linux

package installer

func ensureExecutable(path string) error {
	_ = path
	return nil
}

func ensureInstallExecutablesLinux(installDir string) error {
	_ = installDir
	return nil
}
