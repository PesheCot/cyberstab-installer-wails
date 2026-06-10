//go:build windows

package installer

func DetectClientDir(installDir string) string {
	return DetectClientDirWindows(installDir)
}
