package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const databaseBackupTimeout = 45 * time.Minute

type DatabaseBackupResult struct {
	Path            string
	Message         string
	ServerRestarted bool
}

func readConsoleVersion(installDir string) string {
	base := filepath.Join(serverBundleDir(installDir), "serverconsole")
	if v := readVersionFile(filepath.Join(base, "signature")); v != "" {
		return v
	}
	return ReadInstalledServerVersion(installDir)
}

func databaseBackupRoot(installDir string) string {
	return filepath.Join(serverBundleDir(installDir), "backups")
}

func availableDatabaseBackupPath(installDir string) (string, error) {
	version := readConsoleVersion(installDir)
	if strings.TrimSpace(version) == "" {
		version = "unknown"
	}
	dir := filepath.Join(databaseBackupRoot(installDir), version, "db")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	date := time.Now().Format("2006-01-02--15-04-05")
	for i := 0; i < 1000; i++ {
		name := "db" + date + ".sql"
		if i > 0 {
			name = fmt.Sprintf("db%s--%d.sql", date, i)
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
	}
	return "", fmt.Errorf("не удалось подобрать имя файла резервной копии")
}

func RunDatabaseBackup(installDir string) (DatabaseBackupResult, error) {
	installDir = installDirOrDefault(installDir)
	result := DatabaseBackupResult{}
	st, err := QueryServerStatus(installDir)
	if err != nil {
		return result, err
	}
	serverWasRunning := st.Running
	if st.Running {
		live, liveErr := QueryServerLiveInfo(installDir)
		if liveErr != nil {
			return result, fmt.Errorf("не удалось проверить активных пользователей: %w", liveErr)
		}
		if n := live.SessionCount(); n > 0 {
			return result, fmt.Errorf("для резервного копирования отключите всех пользователей (%d)", n)
		}
		if err := StopServerGracefully(installDir); err != nil {
			return result, fmt.Errorf("не удалось остановить сервер: %w", err)
		}
	}

	backupPath, err := availableDatabaseBackupPath(installDir)
	if err != nil {
		return result, err
	}
	stdin := fmt.Sprintf("backupdb %s\nquit\n", QuoteConsolePath(backupPath))
	out, err := runServerConsoleWithTimeout(installDir, stdin, databaseBackupTimeout)
	out = strings.TrimSpace(out)
	if err != nil {
		result.Path = backupPath
		if out != "" {
			return result, fmt.Errorf("%s", pickDatabaseBackupFailureMessage(out, err.Error()))
		}
		return result, err
	}
	if !isDatabaseBackupSuccessOutput(out) {
		result.Path = backupPath
		return result, fmt.Errorf("%s", pickDatabaseBackupFailureMessage(out, "не удалось создать резервную копию"))
	}

	result.Path = backupPath
	result.Message = pickDatabaseBackupSuccessMessage(out, backupPath)

	if serverWasRunning {
		if err := StartServer(installDir); err != nil {
			result.Message = result.Message + ". Не удалось запустить сервер: " + err.Error()
			return result, nil
		}
		if err := waitForServerRunning(installDir, serverStartWaitTimeout); err != nil {
			result.Message = result.Message + ". Сервер запускается дольше обычного: " + err.Error()
			return result, nil
		}
		result.ServerRestarted = true
		result.Message = result.Message + ". Сервер снова запущен."
	}

	return result, nil
}

func isDatabaseBackupSuccessOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "резервная копия успешно") ||
		strings.Contains(lower, "backup created") ||
		strings.Contains(lower, "backupdb.success")
}

func pickDatabaseBackupSuccessMessage(out, fallbackPath string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && isDatabaseBackupSuccessOutput(line) {
			return line
		}
	}
	return fmt.Sprintf("Резервная копия успешно создана по пути %s", fallbackPath)
}

func pickDatabaseBackupFailureMessage(out, fallback string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "> "))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "не удалось") ||
			strings.Contains(lower, "ошибка") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "отличаются") {
			return line
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "не удалось создать резервную копию базы данных"
}
