//go:build !windows

package system

func LoadNextcloudCredentials() (login, password string, ok bool) {
	return "", "", false
}

func SaveNextcloudCredentials(login, password string, remember bool) error {
	_ = login
	_ = password
	_ = remember
	return ClearNextcloudCredentials()
}

func ClearNextcloudCredentials() error {
	return nil
}

func NextcloudSavedLogin() (string, bool) {
	return "", false
}
