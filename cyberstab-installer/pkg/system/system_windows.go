//go:build windows

package system

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const DefaultInstallDir = `C:\Program Files\Cyberstab`

func QueryServerStatus() (ServerStatus, error) {
	out, err := runCmd(5*time.Second, "schtasks.exe", "/Query", "/TN", scheduledTaskName, "/FO", "LIST", "/V")
	if err != nil {
		return ServerStatus{TaskExists: false, Running: false, Raw: strings.TrimSpace(out)}, nil
	}
	raw := strings.TrimSpace(out)
	running := strings.Contains(strings.ToLower(raw), "status:") && strings.Contains(strings.ToLower(raw), "running")
	return ServerStatus{TaskExists: true, Running: running, Raw: raw}, nil
}

func StartServer() error {
	_, err := runCmd(10*time.Second, "schtasks.exe", "/Run", "/TN", scheduledTaskName)
	return err
}

func StopServer(installDir string) error {
	_ = installDir
	_, err := runCmd(10*time.Second, "schtasks.exe", "/End", "/TN", scheduledTaskName)
	return err
}

func UninstallCyberstab(installDir string) (deferred bool, err error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = DefaultInstallDir
	}
	exePath, _ := os.Executable()
	pid := os.Getpid()

	stopCyberstabWindowsBestEffort()
	_ = removeCyberstabScheduledTasks()
	_ = removeCyberstabShortcuts(installDir)
	_ = removeCyberstabUninstallRegistry(installDir)
	_ = removeCyberstabInstallerArtifactsWindows()

	if rmErr := os.RemoveAll(installDir); rmErr == nil {
		if exePath != "" && !isPathUnder(exePath, installDir) {
			_ = scheduleSelfDelete(pid, exePath, "")
		}
		return false, nil
	}
	_, _ = runCmd(20*time.Second, "cmd.exe", "/C", "rmdir", "/S", "/Q", installDir)
	if _, stErr := os.Stat(installDir); stErr != nil {
		if exePath != "" && !isPathUnder(exePath, installDir) {
			_ = scheduleSelfDelete(pid, exePath, "")
		}
		return false, nil
	}
	_ = scheduleSelfDelete(pid, exePath, installDir)
	return true, fmt.Errorf("папка %s будет удалена после закрытия деинсталлятора", installDir)
}
