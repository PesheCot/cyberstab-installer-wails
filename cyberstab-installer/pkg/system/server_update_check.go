package system

import (
	"fmt"
	"strings"
)

const nextcloudVersionFile = "version.txt"

type NextcloudUpdateCheck struct {
	CurrentVersion string
	RemoteVersion  string
	UpdateRequired bool
	ArchiveName    string
	Message        string
}

func normalizeVersion(v string) string {
	return strings.TrimSpace(v)
}

func versionsEqual(a, b string) bool {
	a = normalizeVersion(a)
	b = normalizeVersion(b)
	return a != "" && b != "" && a == b
}

// CheckNextcloudServerUpdate compares version.txt in the user's login folder on Nextcloud
// with the locally installed server version.
func CheckNextcloudServerUpdate(installDir, login, password string) (NextcloudUpdateCheck, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		return NextcloudUpdateCheck{}, fmt.Errorf("укажите логин и пароль Nextcloud")
	}

	cfg := LoadUpdateSourceConfig()
	if !cfg.Configured() {
		return NextcloudUpdateCheck{}, fmt.Errorf("не настроен Nextcloud")
	}

	remoteVersion, err := FetchNextcloudTextFile(cfg, login, password, nextcloudVersionFile)
	if err != nil {
		return NextcloudUpdateCheck{}, err
	}
	if remoteVersion == "" {
		return NextcloudUpdateCheck{}, fmt.Errorf("файл version.txt в папке %s пустой", login)
	}

	currentVersion := normalizeVersion(ReadInstalledServerVersion(installDir))
	archiveName := ServerUpdateArchiveName(installDir)
	result := NextcloudUpdateCheck{
		CurrentVersion: currentVersion,
		RemoteVersion:  remoteVersion,
		ArchiveName:    archiveName,
	}

	if currentVersion == "" {
		result.UpdateRequired = true
		result.Message = fmt.Sprintf("Не удалось определить текущую версию. На Nextcloud доступна версия %s.", remoteVersion)
		return result, nil
	}

	if versionsEqual(currentVersion, remoteVersion) {
		result.UpdateRequired = false
		result.Message = fmt.Sprintf("Установлена актуальная версия %s. Обновление не требуется.", currentVersion)
		return result, nil
	}

	result.UpdateRequired = true
	result.Message = fmt.Sprintf("Доступно обновление: %s (текущая версия %s).", remoteVersion, currentVersion)
	return result, nil
}
