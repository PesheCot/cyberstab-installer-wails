//go:build linux

package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultInstallDir = "/opt/cyberstab"

const linuxServiceUnit = "cyberstab-server"

func stopCyberstabLinuxBestEffort() {
	_, _ = runCmd(10*time.Second, "systemctl", "stop", linuxServiceUnit)
	time.Sleep(300 * time.Millisecond)
	for _, name := range []string{"CyberstabServerLinux", "serverconsole", "java"} {
		_, _ = runCmd(8*time.Second, "pkill", "-f", name)
	}
	time.Sleep(500 * time.Millisecond)
}

func removeCyberstabLinuxArtifacts(installDir string) {
	_, _ = runCmd(10*time.Second, "systemctl", "disable", linuxServiceUnit)
	_, _ = runCmd(10*time.Second, "systemctl", "stop", linuxServiceUnit)
	_ = os.Remove(filepath.Join("/etc/systemd/system", linuxServiceUnit+".service"))
	_, _ = runCmd(10*time.Second, "systemctl", "daemon-reload")

	home := os.Getenv("HOME")
	if home != "" {
		_ = os.Remove(filepath.Join(home, "Desktop", "cyberstab-client.desktop"))
	}
	for _, dir := range []string{
		filepath.Join("/usr/share/applications"),
		filepath.Join(home, ".local", "share", "applications"),
	} {
		_ = os.Remove(filepath.Join(dir, "cyberstab-client.desktop"))
	}
	_ = os.Remove(filepath.Join("/tmp", "cyberstab-installer.log"))
	_ = installDir
}

func QueryServerStatus() (ServerStatus, error) {
	out, err := runCmd(5*time.Second, "systemctl", "is-active", linuxServiceUnit)
	raw := strings.TrimSpace(out)
	running := err == nil && raw == "active"
	if !running {
		if out2, err2 := runCmd(5*time.Second, "systemctl", "status", linuxServiceUnit); err2 == nil {
			raw = strings.TrimSpace(out2)
		}
	}
	exists := !strings.Contains(strings.ToLower(raw), "could not be found") &&
		!strings.Contains(strings.ToLower(raw), "not-found")
	return ServerStatus{TaskExists: exists, Running: running, Raw: raw}, nil
}

func StartServer() error {
	_, err := runCmd(15*time.Second, "systemctl", "start", linuxServiceUnit)
	return err
}

func StopServer(installDir string) error {
	_ = installDir
	_, err := runCmd(10*time.Second, "systemctl", "stop", linuxServiceUnit)
	return err
}

func UninstallCyberstab(installDir string) (deferred bool, err error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = DefaultInstallDir
	}
	stopCyberstabLinuxBestEffort()
	removeCyberstabLinuxArtifacts(installDir)
	if rmErr := os.RemoveAll(installDir); rmErr == nil {
		return false, nil
	}
	return true, fmt.Errorf("папка %s будет удалена после закрытия деинсталлятора", installDir)
}
