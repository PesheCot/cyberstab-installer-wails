//go:build !linux

package installer

import "fmt"

func DetectLinuxOS() (LinuxOSInfo, error) {
	return LinuxOSInfo{}, fmt.Errorf("not linux")
}

func FindAvailablePostgresPackages() ([]PostgresPackageOption, error) {
	return nil, fmt.Errorf("not linux")
}

func InstallPostgresPackage(version string) error {
	return fmt.Errorf("not linux")
}

func IsPackageManagerPostgresInstall(selection string) bool {
	return false
}
