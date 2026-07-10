//go:build windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const DefaultInstallDir = `C:\Program Files\Cyberstab`

func QueryServerStatus(installDir string) (ServerStatus, error) {
	installDir = installDirOrDefault(installDir)
	ports := LoadServerPorts(installDir)

	out, err := runCmd(5*time.Second, "schtasks.exe", "/Query", "/TN", scheduledTaskName, "/FO", "LIST", "/V")
	taskExists := err == nil
	raw := strings.TrimSpace(out)
	if !taskExists {
		raw = strings.TrimSpace(fmt.Sprintf("%v", err))
	}

	running := isLocalTCPPortOpen(ports.NetworkPort, 500*time.Millisecond)
	if running {
		raw = fmt.Sprintf("network.portnumber=%d: listening\n%s", ports.NetworkPort, raw)
	} else {
		raw = fmt.Sprintf("network.portnumber=%d: not listening\n%s", ports.NetworkPort, raw)
	}

	return ServerStatus{
		TaskExists:     taskExists,
		Running:        running,
		Raw:            raw,
		NetworkPort:    ports.NetworkPort,
		ManagementPort: ports.ManagementPort,
		PropertiesPath: ports.PropertiesPath,
	}, nil
}

func findServerExecutable(installDir string) (string, error) {
	installDir = installDirOrDefault(installDir)
	serverExe := filepath.Join(installDir, "CyberstabServerWindows", "server", "CyberstabServerWindows.exe")
	serverConsole := filepath.Join(installDir, "CyberstabServerWindows", "serverconsole", "serverconsole.exe")
	if _, err := os.Stat(serverExe); err == nil {
		return serverExe, nil
	}
	if _, err := os.Stat(serverConsole); err == nil {
		return serverConsole, nil
	}
	return "", fmt.Errorf("исполняемый файл сервера не найден в %s", installDir)
}

func startServerViaScheduledTask() error {
	var lastErr error
	for i := 0; i < 3; i++ {
		_, err := runCmd(10*time.Second, "schtasks.exe", "/Run", "/TN", scheduledTaskName)
		if err == nil {
			time.Sleep(2 * time.Second)
			return nil
		}
		lastErr = err
		if i < 2 {
			time.Sleep(time.Second)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("не удалось запустить задачу %s", scheduledTaskName)
	}
	return lastErr
}

func startServerDirect(installDir string) error {
	serverPath, err := findServerExecutable(installDir)
	if err != nil {
		return err
	}
	cmd := exec.Command(serverPath)
	cmd.Dir = filepath.Dir(serverPath)
	hideCmdDetached(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить %s: %w", filepath.Base(serverPath), err)
	}
	return nil
}

func StartServer(installDir string) error {
	installDir = installDirOrDefault(installDir)
	st, err := QueryServerStatus(installDir)
	if err == nil && st.Running {
		return nil
	}
	if st.TaskExists && !st.Running {
		_, _ = runCmd(10*time.Second, "schtasks.exe", "/End", "/TN", scheduledTaskName)
		time.Sleep(500 * time.Millisecond)
	}

	taskErr := startServerViaScheduledTask()
	if taskErr == nil {
		if err := waitForServerRunning(installDir, 45*time.Second); err == nil {
			return nil
		}
	}

	if err := startServerDirect(installDir); err != nil {
		if taskErr != nil {
			return fmt.Errorf("%v; прямой запуск: %w", taskErr, err)
		}
		return err
	}
	if err := waitForServerRunning(installDir, serverStartWaitTimeout); err != nil {
		return err
	}
	return nil
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
