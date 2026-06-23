//go:build !linux

package installer

func ConfigureLinuxPlatform() error {
	return nil
}

func ensureClientLogDirsLinux(installDir string) {}
