//go:build windows

package system

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type savedNextcloudCredentials struct {
	Login             string `json:"login"`
	PasswordEncrypted string `json:"passwordEncrypted"`
}

func nextcloudCredentialsPath() string {
	return filepath.Join(managerDataDir(), "nextcloud-credentials.json")
}

func LoadNextcloudCredentials() (login, password string, ok bool) {
	b, err := os.ReadFile(nextcloudCredentialsPath())
	if err != nil {
		return "", "", false
	}
	var saved savedNextcloudCredentials
	if err := json.Unmarshal(b, &saved); err != nil {
		return "", "", false
	}
	login = strings.TrimSpace(saved.Login)
	if login == "" || strings.TrimSpace(saved.PasswordEncrypted) == "" {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(saved.PasswordEncrypted)
	if err != nil {
		return "", "", false
	}
	plain, err := dpapiUnprotect(raw)
	if err != nil {
		return "", "", false
	}
	return login, string(plain), true
}

func SaveNextcloudCredentials(login, password string, remember bool) error {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)
	if !remember {
		return ClearNextcloudCredentials()
	}
	if login == "" || password == "" {
		return errors.New("укажите логин и пароль Nextcloud")
	}
	protected, err := dpapiProtect([]byte(password))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(managerDataDir(), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(savedNextcloudCredentials{
		Login:             login,
		PasswordEncrypted: base64.StdEncoding.EncodeToString(protected),
	}, "", "  ")
	if err != nil {
		return err
	}
	tmp := nextcloudCredentialsPath() + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, nextcloudCredentialsPath())
}

func ClearNextcloudCredentials() error {
	err := os.Remove(nextcloudCredentialsPath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func NextcloudSavedLogin() (string, bool) {
	login, _, ok := LoadNextcloudCredentials()
	return login, ok
}
