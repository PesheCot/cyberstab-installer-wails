//go:build linux

package installer

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	linuxServiceName = "cyberstab-server.service"
	linuxServiceUnit = "cyberstab-server"
)

func (e *Engine) writeEmbeddedUninstallerLinux() error {
	if len(e.UninstallerData) == 0 {
		return nil
	}
	installDir := installDirOrDefault(e.InstallDir)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to prepare install dir for uninstaller: %w", err)
	}
	unPath := filepath.Join(installDir, "cyberstab-uninstaller")
	if err := os.WriteFile(unPath, e.UninstallerData, 0755); err != nil {
		return fmt.Errorf("failed to write uninstaller to %s: %w", unPath, err)
	}
	log.Printf("[INSTALL] Uninstaller written: %s (%d bytes)", unPath, len(e.UninstallerData))
	return nil
}

func finishInstallLinux(e *Engine) error {
	if err := e.writeEmbeddedUninstallerLinux(); err != nil {
		return err
	}

	if e.Options.Components.InstallClients {
		_ = setClientFolderPermissionsLinux(e.InstallDir)
		_ = createClientDesktopEntryLinux(e.InstallDir)
	}

	if !(e.Options.Components.InstallServer || e.Options.Components.InstallDB) {
		return nil
	}

	if e.ProgressEmitter != nil {
		e.ProgressEmitter(90, "Настройка автозапуска сервера…")
	}

	serverPath := findServerExecutableLinux(e.InstallDir)
	if serverPath == "" {
		if e.ProgressEmitter != nil {
			e.ProgressEmitter(90, "Сервер не найден")
		}
		return nil
	}

	if err := ensureServerAutostartLinux(e.InstallDir, serverPath); err != nil {
		if e.ProgressEmitter != nil {
			e.ProgressEmitter(92, "Автозапуск: "+err.Error())
		}
	} else if e.ProgressEmitter != nil {
		e.ProgressEmitter(93, "Автозапуск создан")
	}

	if e.ProgressEmitter != nil {
		e.ProgressEmitter(95, "Запуск сервера…")
	}
	if err := startServerLinux(); err != nil {
		if e.ProgressEmitter != nil {
			e.ProgressEmitter(96, "Ошибка запуска: "+err.Error())
		}
		return fmt.Errorf("не удалось запустить сервер: %w", err)
	}

	if e.ProgressEmitter != nil {
		e.ProgressEmitter(98, "Ожидание инициализации сервера…")
	}
	if err := waitForServerReadyLinux(e.InstallDir, 4*time.Minute); err != nil {
		if e.ProgressEmitter != nil {
			e.ProgressEmitter(99, "Сервер не готов: "+err.Error())
		}
		return err
	}
	if e.ProgressEmitter != nil {
		e.ProgressEmitter(99, "Сервер инициализирован")
	}
	return nil
}

func findServerExecutableLinux(installDir string) string {
	candidates := []string{
		filepath.Join(installDir, "CyberstabServerLinux", "server", "CyberstabServerLinux"),
		filepath.Join(installDir, "CyberstabServerLinux", "serverconsole", "serverconsole"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	var hit string
	_ = filepath.WalkDir(filepath.Join(installDir, "CyberstabServerLinux"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.Contains(name, "cyberstabserver") || name == "serverconsole" {
			hit = path
			return filepath.SkipAll
		}
		return nil
	})
	return hit
}

func ensureServerAutostartLinux(installDir, serverPath string) error {
	if _, err := os.Stat(serverPath); err != nil {
		return fmt.Errorf("сервер не найден: %w", err)
	}
	unitPath := filepath.Join("/etc/systemd/system", linuxServiceUnit+".service")
	workDir := filepath.Dir(serverPath)
	content := fmt.Sprintf(`[Unit]
Description=Cyberstab Server
After=network.target postgresql.service

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, workDir, serverPath)

	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("не удалось записать unit %s: %w", unitPath, err)
	}
	_ = runLinuxCmd("systemctl", "daemon-reload")
	_ = runLinuxCmd("systemctl", "enable", linuxServiceUnit)
	return nil
}

func startServerLinux() error {
	for i := 0; i < 3; i++ {
		if err := runLinuxCmd("systemctl", "start", linuxServiceUnit); err == nil {
			time.Sleep(2 * time.Second)
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("systemctl start %s failed", linuxServiceUnit)
}

func waitForServerReadyLinux(installDir string, timeout time.Duration) error {
	logPath := filepath.Join(installDir, "CyberstabServerLinux", "log", "server", "error_log.log")
	return waitForLogLineLinux(logPath, "[main] NetworkMessageDispatcher started", timeout)
}

func waitForLogLineLinux(logPath, needle string, timeout time.Duration) error {
	if strings.TrimSpace(logPath) == "" {
		return fmt.Errorf("log path is empty")
	}
	deadline := time.Now().Add(timeout)
	var lastSize int64 = -1
	var stableCount int
	for time.Now().Before(deadline) {
		fi, err := os.Stat(logPath)
		if err != nil {
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		size := fi.Size()
		if lastSize == size {
			stableCount++
		} else {
			stableCount = 0
		}
		lastSize = size
		const maxTail = int64(512 * 1024)
		start := int64(0)
		if size > maxTail {
			start = size - maxTail
		}
		f, err := os.Open(logPath)
		if err != nil {
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		_, _ = f.Seek(start, 0)
		b, _ := io.ReadAll(f)
		_ = f.Close()
		if bytes.Contains(b, []byte(needle)) {
			return nil
		}
		if stableCount > 5 {
			time.Sleep(1800 * time.Millisecond)
		} else {
			time.Sleep(900 * time.Millisecond)
		}
	}
	return fmt.Errorf("таймаут ожидания строки в логе: %s", needle)
}

func stopCyberstabProcessesLinux() error {
	_ = runLinuxCmd("systemctl", "stop", linuxServiceUnit)
	time.Sleep(300 * time.Millisecond)
	for _, name := range []string{"CyberstabServerLinux", "serverconsole", "CyberstabClientLinux"} {
		_ = runLinuxCmd("pkill", "-f", name)
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func setClientFolderPermissionsLinux(installDir string) error {
	var dirs []string
	for _, name := range []string{"CyberstabClientLinux64", "CyberstabClientLinux32"} {
		p := filepath.Join(installDir, name)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			dirs = append(dirs, p)
		}
	}
	if len(dirs) == 0 {
		if dir := DetectClientDirLinux(installDir); dir != "" {
			dirs = append(dirs, dir)
		}
	}
	for _, dir := range dirs {
		_ = runLinuxCmd("chmod", "-R", "a+rx", dir)
	}
	return nil
}

func createClientDesktopEntryLinux(installDir string) error {
	clientDir := DetectClientDirLinux(installDir)
	if clientDir == "" {
		return fmt.Errorf("client directory not found")
	}
	targetExe := FindClientExeBestEffort(clientDir)
	if targetExe == "" {
		return fmt.Errorf("client executable not found in %s", clientDir)
	}
	desktopDir := filepath.Join(os.Getenv("HOME"), "Desktop")
	if xdg := strings.TrimSpace(os.Getenv("XDG_DESKTOP_DIR")); xdg != "" {
		desktopDir = xdg
	}
	if err := os.MkdirAll(desktopDir, 0755); err != nil {
		return err
	}
	entryPath := filepath.Join(desktopDir, "cyberstab-client.desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Киберстаб
Comment=Киберстаб Клиент
Exec=%s
Path=%s
Terminal=false
`, targetExe, filepath.Dir(targetExe))
	if err := os.WriteFile(entryPath, []byte(content), 0755); err != nil {
		return err
	}
	return nil
}

func DetectClientDirLinux(installDir string) string {
	if is64BitLinux() {
		return filepath.Join(installDir, "CyberstabClientLinux64")
	}
	return filepath.Join(installDir, "CyberstabClientLinux32")
}

func DetectClientDir(installDir string) string {
	return DetectClientDirLinux(installDir)
}

func runLinuxCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(b.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}
