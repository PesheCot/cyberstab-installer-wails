package system

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const serverFullUpdateTimeout = 2 * time.Hour
const serverStartWaitTimeout = 3 * time.Minute
const serverStartPollInterval = 2 * time.Second
const consoleSelfUpdateTimeout = 45 * time.Minute

func QuoteConsolePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return `""`
	}
	if !strings.ContainsAny(p, " \t\"") {
		return p
	}
	return `"` + strings.ReplaceAll(p, `"`, `\"`) + `"`
}

func ensureServerReadyForUpdate(installDir string, force bool) error {
	if force {
		return nil
	}
	installDir = installDirOrDefault(installDir)
	st, err := QueryServerStatus(installDir)
	if err != nil {
		return err
	}
	if !st.Running {
		return nil
	}
	live, err := QueryServerLiveInfo(installDir)
	if err != nil {
		return fmt.Errorf("не удалось проверить активных пользователей: %w", err)
	}
	if n := live.SessionCount(); n > 0 {
		return fmt.Errorf("нельзя обновить сервер: подключено пользователей — %d. Отключите всех на вкладке «Пользователи»", n)
	}
	return nil
}

func RunServerFullUpdate(installDir, archivePath string, force bool) (string, error) {
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return "", fmt.Errorf("укажите путь к архиву обновления")
	}
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("файл обновления не найден: %s", archivePath)
		}
		return "", err
	}
	if !force {
		if err := ensureServerReadyForUpdate(installDir, force); err != nil {
			return "", err
		}
	}

	cmdName := "fullupdate"
	if force {
		cmdName = "fullupdateforce"
	}
	stdin := fmt.Sprintf("%s %s\nquit\n", cmdName, QuoteConsolePath(archivePath))
	out, err := runServerConsoleWithTimeout(installDir, stdin, serverFullUpdateTimeout)
	out = strings.TrimSpace(out)
	if err != nil {
		if out != "" {
			return out, fmt.Errorf("%s", pickUpdateFailureMessage(out, err.Error()))
		}
		return "", err
	}
	if isUpdateSuccessOutput(out) {
		return pickUpdateSuccessMessage(out), nil
	}
	if isUpdateFailureOutput(out) {
		return out, fmt.Errorf("%s", pickUpdateFailureMessage(out, "обновление завершилось с ошибкой"))
	}
	if out == "" {
		return "", fmt.Errorf("обновление завершилось без ответа консоли")
	}
	return out, fmt.Errorf("%s", pickUpdateFailureMessage(out, "обновление завершилось с ошибкой"))
}

func waitForServerRunning(installDir string, timeout time.Duration) error {
	installDir = installDirOrDefault(installDir)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := QueryServerStatus(installDir)
		if err == nil && st.Running {
			ports := LoadServerPorts(installDir)
			if isLocalTCPPortOpen(ports.ManagementPort, 500*time.Millisecond) {
				return nil
			}
		}
		time.Sleep(serverStartPollInterval)
	}
	return fmt.Errorf("сервер не запустился в отведённое время")
}

func RunConsoleSelfUpdate(installDir string) (string, error) {
	out, err := runServerConsoleWithTimeout(installDir, "selfupdate\nquit\n", consoleSelfUpdateTimeout)
	out = strings.TrimSpace(out)
	if err != nil {
		if out != "" {
			return out, fmt.Errorf("%s", pickSelfUpdateFailureMessage(out, err.Error()))
		}
		return "", err
	}
	if isSelfUpdateSuccessOutput(out) {
		return pickSelfUpdateSuccessMessage(out), nil
	}
	if isSelfUpdateFailureOutput(out) {
		return out, fmt.Errorf("%s", pickSelfUpdateFailureMessage(out, "обновление консоли завершилось с ошибкой"))
	}
	if out == "" {
		return "", fmt.Errorf("обновление консоли завершилось без ответа")
	}
	return out, fmt.Errorf("%s", pickSelfUpdateFailureMessage(out, "обновление консоли завершилось с ошибкой"))
}

func RunCompleteServerUpdate(installDir, archivePath string, force bool) (string, error) {
	msg, err := RunServerFullUpdate(installDir, archivePath, force)
	if err != nil {
		return msg, err
	}
	if err := StartServer(installDir); err != nil {
		return msg, fmt.Errorf("сервер обновлён, но не удалось запустить: %w", err)
	}
	if err := waitForServerRunning(installDir, serverStartWaitTimeout); err != nil {
		return msg, fmt.Errorf("сервер обновлён, запуск инициирован, но %w", err)
	}
	selfMsg, err := RunConsoleSelfUpdate(installDir)
	if err != nil {
		combined := msg
		if strings.TrimSpace(selfMsg) != "" {
			combined = combined + "\n" + selfMsg
		}
		return combined, fmt.Errorf("сервер обновлён и запущен, но обновление консоли не удалось: %w", err)
	}
	_ = selfMsg
	return "Полное обновление завершено: сервер обновлён, запущен, консоль обновлена.", nil
}

func isSelfUpdateSuccessOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "загрузка завершена") ||
		strings.Contains(lower, "download done") ||
		strings.Contains(lower, "запускается обновление")
}

func isSelfUpdateFailureOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "selfupdate.error") ||
		strings.Contains(lower, "произошла ошибка") ||
		strings.Contains(lower, "прервано") ||
		strings.Contains(lower, "interrupted")
}

func pickSelfUpdateSuccessMessage(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isSelfUpdateSuccessOutput(line) {
			return line
		}
	}
	return "Консоль обновлена."
}

func pickSelfUpdateFailureMessage(out, fallback string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "> "))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "ошибка") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "прервано") {
			return line
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "обновление консоли завершилось с ошибкой"
}

func isUpdateSuccessOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "успешно обновлен") ||
		strings.Contains(lower, "succesfully updated") ||
		strings.Contains(lower, "successfully updated")
}

func isUpdateFailureOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "прошло с ошибкой") ||
		strings.Contains(lower, "with error") ||
		strings.Contains(lower, "updateverify.fail") ||
		strings.Contains(lower, "некорректный формат")
}

func pickUpdateSuccessMessage(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isUpdateSuccessOutput(line) {
			return line
		}
	}
	return "Сервер успешно обновлён."
}

func pickUpdateFailureMessage(out, fallback string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "> "))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "прошло с ошибкой") ||
			strings.Contains(lower, "with error") ||
			strings.Contains(lower, "не найден") ||
			strings.Contains(lower, "некоррект") ||
			strings.Contains(lower, "подключен") {
			return line
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "Обновление сервера прошло с ошибкой"
}

func RunServerUpdateFromNextcloud(installDir, login, password string, remember, force bool) (string, string, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		return "", "", fmt.Errorf("укажите логин и пароль Nextcloud")
	}
	cfg := LoadUpdateSourceConfig()
	if !cfg.Configured() {
		return "", "", fmt.Errorf("не настроен Nextcloud: создайте файл %s", updateConfigPath())
	}
	check, err := CheckNextcloudServerUpdate(installDir, login, password)
	if err != nil {
		return "", "", err
	}
	if !check.UpdateRequired {
		return "", "", fmt.Errorf("%s", check.Message)
	}
	if err := SaveNextcloudCredentials(login, password, remember); err != nil {
		return "", "", err
	}
	archivePath, err := DownloadNextcloudServerUpdate(cfg, login, password, check.ArchiveName)
	if err != nil {
		return "", "", err
	}
	msg, err := RunCompleteServerUpdate(installDir, archivePath, force)
	return msg, archivePath, err
}

func RunServerUpdateFromLocalPath(installDir, archivePath string, force bool) (string, error) {
	return RunCompleteServerUpdate(installDir, archivePath, force)
}
