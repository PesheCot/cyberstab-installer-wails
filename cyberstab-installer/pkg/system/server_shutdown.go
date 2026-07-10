package system

import (
	"fmt"
	"time"
)

const (
	serverStopWaitTimeout        = 4 * time.Minute
	serverShutdownConsoleTimeout = 3 * time.Minute
)

func waitForServerStopped(installDir string, timeout time.Duration) error {
	installDir = installDirOrDefault(installDir)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := QueryServerStatus(installDir)
		if err == nil && !st.Running {
			return nil
		}
		time.Sleep(serverStartPollInterval)
	}
	return fmt.Errorf("сервер не остановился в отведённое время")
}

// StopServerGracefully stops the JVM server via console shutdown, then falls back to schtasks/taskkill.
func StopServerGracefully(installDir string) error {
	installDir = installDirOrDefault(installDir)
	st, err := QueryServerStatus(installDir)
	if err != nil {
		return err
	}
	if !st.Running {
		return nil
	}

	InvalidateServerLiveInfoCache()
	_, _ = runServerConsoleWithTimeout(installDir, "shutdown\nquit\n", serverShutdownConsoleTimeout)
	if err := waitForServerStopped(installDir, serverStopWaitTimeout); err == nil {
		return nil
	}

	_ = StopServer(installDir)
	if err := waitForServerStopped(installDir, 90*time.Second); err == nil {
		return nil
	}

	_ = taskkillBestEffort("CyberstabServerWindows.exe")
	return waitForServerStopped(installDir, 60*time.Second)
}
