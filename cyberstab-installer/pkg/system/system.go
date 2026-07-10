package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ServerStatus struct {
	TaskExists     bool
	Running        bool
	Raw            string
	NetworkPort    int
	ManagementPort int
	PropertiesPath string
}

// NOTE: These names are project-specific; adjust if your scheduled task names differ.
const scheduledTaskName = "CyberstabServer"

func stopCyberstabWindowsBestEffort() {
	if runtime.GOOS != "windows" {
		return
	}
	// Stop scheduled task first (releases most file locks).
	_, _ = runCmd(10*time.Second, "schtasks.exe", "/End", "/TN", scheduledTaskName)
	time.Sleep(300 * time.Millisecond)

	// Then kill known executables (best-effort).
	_ = taskkillBestEffort("CyberstabServerWindows.exe")
	_ = taskkillBestEffort("serverconsole.exe")
	_ = taskkillBestEffort("CyberstabClientWindows.exe")
	// Cyberstab uses Java; as last resort kill java.exe (may be too broad, but needed for uninstall).
	_ = taskkillBestEffort("java.exe")
	time.Sleep(500 * time.Millisecond)
}

func taskkillBestEffort(imageName string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if strings.TrimSpace(imageName) == "" {
		return nil
	}
	_, _ = runCmd(12*time.Second, "taskkill.exe", "/F", "/IM", imageName, "/T")
	return nil
}

func removeCyberstabInstallerArtifactsWindows() error {
	// Best-effort cleanup of installer artifacts outside installDir:
	// - WebView2 user data dir (created to avoid Edge "cannot read/write" when running elevated)
	// - installer log files
	base := os.Getenv("ProgramData")
	if strings.TrimSpace(base) == "" {
		base = `C:\ProgramData`
	}
	_ = os.RemoveAll(filepath.Join(base, "CyberstabInstaller"))
	_ = os.Remove(filepath.Join(base, "cyberstab-installer.log"))
	_ = os.Remove(filepath.Join(os.TempDir(), "cyberstab-installer.log"))
	return nil
}

func removeCyberstabScheduledTasks() error {
	// If task is running, stop it first.
	_, _ = runCmd(10*time.Second, "schtasks.exe", "/End", "/TN", scheduledTaskName)
	time.Sleep(200 * time.Millisecond)

	// Delete the known task name first.
	_, _ = runCmd(10*time.Second, "schtasks.exe", "/Delete", "/TN", scheduledTaskName, "/F")

	// Then try to find and remove any remaining tasks that contain "Cyberstab".
	out, err := runCmd(12*time.Second, "schtasks.exe", "/Query", "/FO", "CSV", "/NH")
	if err != nil {
		// best-effort
		return nil
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV first field: "TaskName",...
		// Keep it simple: extract between first two quotes.
		if !strings.HasPrefix(line, "\"") {
			continue
		}
		parts := strings.SplitN(line, "\",", 2)
		if len(parts) < 1 {
			continue
		}
		taskName := strings.Trim(parts[0], "\"")
		if strings.Contains(strings.ToLower(taskName), "cyberstab") {
			_, _ = runCmd(10*time.Second, "schtasks.exe", "/Delete", "/TN", taskName, "/F")
		}
	}
	return nil
}

func removeCyberstabShortcuts(installDir string) error {
	// Remove common shortcut locations (best-effort).
	candidates := []string{
		filepath.Join(os.Getenv("Public"), "Desktop"),
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop"),
		filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs"),
		filepath.Join(os.Getenv("AppData"), "Microsoft", "Windows", "Start Menu", "Programs"),
	}
	for _, base := range candidates {
		if strings.TrimSpace(base) == "" {
			continue
		}
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			name := strings.ToLower(d.Name())
			if !strings.HasSuffix(name, ".lnk") {
				return nil
			}
			if strings.Contains(name, "cyberstab") {
				_ = os.Remove(path)
				return nil
			}
			return nil
		})
	}
	return nil
}

func isPathUnder(child, parent string) bool {
	c, err1 := filepath.Abs(child)
	p, err2 := filepath.Abs(parent)
	if err1 != nil || err2 != nil {
		return false
	}
	c = strings.ToLower(filepath.Clean(c))
	p = strings.ToLower(filepath.Clean(p))
	return strings.HasPrefix(c, p+strings.ToLower(string(os.PathSeparator))) || c == p
}

func scheduleSelfDelete(pid int, exePath string, installDir string) error {
	// Create a temporary .cmd that waits for this PID to exit, then deletes installDir and/or exePath.
	tmp := os.TempDir()
	script := filepath.Join(tmp, "cyberstab_uninstall_"+strconv.Itoa(pid)+".cmd")

	// Quote paths for cmd.
	q := func(s string) string {
		if s == "" {
			return ""
		}
		// Windows paths should not contain quotes; keep quoting simple and predictable.
		return `"` + s + `"`
	}

	var lines []string
	lines = append(lines, "@echo off")
	lines = append(lines, "set PID="+strconv.Itoa(pid))
	lines = append(lines, ":wait")
	lines = append(lines, "tasklist /FI \"PID eq %PID%\" | find \"%PID%\" >nul")
	lines = append(lines, "if %errorlevel%==0 (timeout /t 1 /nobreak >nul & goto wait)")
	// Try hard to remove the install directory and the uninstaller exe.
	// Important: even if the uninstaller lives inside installDir, installDir removal may partially fail.
	// In that case we still want to explicitly delete exePath after the process exits.
	if strings.TrimSpace(installDir) != "" {
		lines = append(lines, "rmdir /S /Q "+q(installDir))
		lines = append(lines, "timeout /t 1 /nobreak >nul")
		// Second attempt sometimes succeeds after a short delay.
		lines = append(lines, "rmdir /S /Q "+q(installDir))
	}
	if strings.TrimSpace(exePath) != "" {
		lines = append(lines, "del /F /Q "+q(exePath))
		lines = append(lines, "timeout /t 1 /nobreak >nul")
		// If the exe was inside installDir, retry directory removal after deleting the exe.
		if strings.TrimSpace(installDir) != "" {
			lines = append(lines, "rmdir /S /Q "+q(installDir))
		}
	}
	lines = append(lines, "del /F /Q \"%~f0\"")
	content := strings.Join(lines, "\r\n") + "\r\n"

	if err := os.WriteFile(script, []byte(content), 0600); err != nil {
		return err
	}

	// Fire-and-forget, hidden window.
	cmd := exec.Command("cmd.exe", "/C", script)
	hideCmd(cmd)
	return cmd.Start()
}

func runCmd(timeout time.Duration, exe string, args ...string) (string, error) {
	cmd := exec.Command(exe, args...)
	hideCmd(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		out := stdout.String()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return out, fmt.Errorf("%s", msg)
		}
		return out, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return stdout.String(), fmt.Errorf("timeout")
	}
}

