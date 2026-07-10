package system

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var signatureVersionRe = regexp.MustCompile(`;0:([0-9]+(?:\.[0-9]+)*)$`)

func serverBundleDir(installDir string) string {
	installDir = installDirOrDefault(installDir)
	if runtime.GOOS == "windows" {
		return filepath.Join(installDir, "CyberstabServerWindows")
	}
	return filepath.Join(installDir, "CyberstabServerLinux")
}

// ReadInstalledServerVersion returns the packaged server version from local install files.
func ReadInstalledServerVersion(installDir string) string {
	base := serverBundleDir(installDir)
	candidates := []string{
		filepath.Join(base, "server", "ClientDistr", "version.txt"),
		filepath.Join(base, "server", "signature"),
		filepath.Join(base, "serverconsole", "signature"),
	}
	for _, path := range candidates {
		if v := readVersionFile(path); v != "" {
			return v
		}
	}
	return ""
}

func readVersionFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return ""
	}
	if strings.EqualFold(filepath.Base(path), "version.txt") {
		return text
	}
	if m := signatureVersionRe.FindStringSubmatch(text); len(m) == 2 {
		return m[1]
	}
	return ""
}
