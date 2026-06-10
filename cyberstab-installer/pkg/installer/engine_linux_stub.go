//go:build !linux

package installer

import "os"

func setClientFolderPermissionsLinux(installDir string) error {
	_ = installDir
	return nil
}

func stopCyberstabProcessesLinux() error {
	return nil
}

func runLinuxCmd(name string, args ...string) error {
	_ = name
	_ = args
	return nil
}

func isLikelyExecutable(path string, d os.DirEntry) bool {
	_ = path
	_ = d
	return false
}
