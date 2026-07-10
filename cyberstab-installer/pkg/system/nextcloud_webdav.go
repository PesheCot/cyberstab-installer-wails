package system

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const nextcloudDownloadTimeout = 45 * time.Minute

func nextcloudFolderURL(baseURL, login, folder string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	login = strings.TrimSpace(login)
	if baseURL == "" {
		return "", fmt.Errorf("не задан адрес Nextcloud")
	}
	if login == "" {
		return "", fmt.Errorf("укажите логин Nextcloud")
	}
	parts := []string{baseURL, "remote.php", "dav", "files", url.PathEscape(login)}
	for _, segment := range strings.Split(folder, "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		parts = append(parts, url.PathEscape(segment))
	}
	return strings.Join(parts, "/") + "/", nil
}

func nextcloudHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func nextcloudRequest(method, rawURL, login, password string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(login, password)
	req.Header.Set("User-Agent", "Cyberstab-Manager/1.0")
	return req, nil
}

func updateFolderPath(cfg UpdateSourceConfig, folderName string) string {
	folderName = strings.Trim(folderName, "/")
	if folderName == "" {
		return strings.Trim(cfg.UpdatesFolder, "/")
	}
	if cfg.UpdatesFolder == "" {
		return folderName
	}
	return cfg.UpdatesFolder + "/" + folderName
}

func nextcloudFileURL(cfg UpdateSourceConfig, login, folderName, fileName string) (string, error) {
	rel := updateFolderPath(cfg, folderName)
	if rel == "" && strings.TrimSpace(folderName) != "" {
		return "", fmt.Errorf("укажите папку с обновлением")
	}
	folderURL, err := nextcloudFolderURL(cfg.BaseURL, login, rel)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(folderURL, "/") + "/" + url.PathEscape(fileName), nil
}

func verifyNextcloudFileExists(fileURL, login, password string) error {
	req, err := nextcloudRequest(http.MethodHead, fileURL, login, password, nil)
	if err != nil {
		return err
	}
	resp, err := nextcloudHTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("не удалось проверить архив: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("неверный логин или пароль Nextcloud")
	case http.StatusNotFound:
		return fmt.Errorf("архив не найден на сервере")
	default:
		return fmt.Errorf("проверка файла завершилась с кодом %d", resp.StatusCode)
	}
}

// FetchNextcloudTextFile reads a small text file from the folder named after the user's login.
func FetchNextcloudTextFile(cfg UpdateSourceConfig, login, password, fileName string) (string, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)
	fileName = strings.TrimSpace(fileName)
	if login == "" || password == "" {
		return "", fmt.Errorf("укажите логин и пароль Nextcloud")
	}
	if fileName == "" {
		return "", fmt.Errorf("не указан файл")
	}

	text, err := fetchNextcloudTextAt(cfg, login, password, login, fileName)
	if err == nil {
		return text, nil
	}
	if !isNextcloudNotFound(err) {
		return "", err
	}
	return fetchNextcloudTextAt(cfg, login, password, "", fileName)
}

func fetchNextcloudTextAt(cfg UpdateSourceConfig, login, password, folderName, fileName string) (string, error) {
	fileURL, err := nextcloudFileURL(cfg, login, folderName, fileName)
	if err != nil {
		return "", err
	}
	req, err := nextcloudRequest(http.MethodGet, fileURL, login, password, nil)
	if err != nil {
		return "", err
	}
	resp, err := nextcloudHTTPClient(60 * time.Second).Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения %s: %w", fileName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("неверный логин или пароль Nextcloud")
	}
	if resp.StatusCode == http.StatusNotFound {
		if folderName == "" {
			return "", fmt.Errorf("файл %s не найден", fileName)
		}
		return "", fmt.Errorf("файл %s не найден в папке %s", fileName, folderName)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("чтение %s завершилось с кодом %d", fileName, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func isNextcloudNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "не найден") || strings.Contains(msg, "not found")
}

// DownloadNextcloudServerUpdate downloads the server update archive from the login folder.
func DownloadNextcloudServerUpdate(cfg UpdateSourceConfig, login, password, archiveName string) (string, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)
	archiveName = strings.TrimSpace(archiveName)
	if login == "" || password == "" {
		return "", fmt.Errorf("укажите логин и пароль Nextcloud")
	}
	if archiveName == "" {
		return "", fmt.Errorf("не определён архив обновления")
	}

	path, err := downloadNextcloudArchiveAt(cfg, login, password, login, archiveName)
	if err == nil {
		return path, nil
	}
	if !isNextcloudNotFound(err) {
		return "", err
	}
	return downloadNextcloudArchiveAt(cfg, login, password, "", archiveName)
}

func downloadNextcloudArchiveAt(cfg UpdateSourceConfig, login, password, folderName, archiveName string) (string, error) {
	fileURL, err := nextcloudFileURL(cfg, login, folderName, archiveName)
	if err != nil {
		return "", err
	}
	if err := verifyNextcloudFileExists(fileURL, login, password); err != nil {
		return "", err
	}
	return downloadNextcloudFile(fileURL, archiveName, login, password)
}

func downloadNextcloudFile(fileURL, fileName, login, password string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "", fmt.Errorf("пустое имя файла")
	}
	req, err := nextcloudRequest(http.MethodGet, fileURL, login, password, nil)
	if err != nil {
		return "", err
	}
	resp, err := nextcloudHTTPClient(nextcloudDownloadTimeout).Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка скачивания: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("неверный логин или пароль Nextcloud")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("скачивание завершилось с кодом %d", resp.StatusCode)
	}

	destDir := filepath.Join(managerDataDir(), "downloads")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	destPath := filepath.Join(destDir, fileName)
	tmpPath := destPath + ".part"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("ошибка записи файла: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return destPath, nil
}
