package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type UpdateSourceConfig struct {
	BaseURL       string `json:"baseURL"`
	UpdatesFolder string `json:"updatesFolder"`
}

func managerDataDir() string {
	base := os.Getenv("ProgramData")
	if strings.TrimSpace(base) == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "Cyberstab", "manager")
}

func updateConfigPath() string {
	return filepath.Join(managerDataDir(), "updates-config.json")
}

// UpdateConfigPath returns the path to updates-config.json in ProgramData.
func UpdateConfigPath() string {
	return updateConfigPath()
}

const defaultNextcloudBaseURL = "https://webpublic.yarsec.ru"

func LoadUpdateSourceConfig() UpdateSourceConfig {
	cfg := UpdateSourceConfig{
		BaseURL: defaultNextcloudBaseURL,
	}
	if v := strings.TrimSpace(os.Getenv("CYBERSTAB_NEXTCLOUD_URL")); v != "" {
		cfg.BaseURL = strings.TrimRight(v, "/")
	}
	b, err := os.ReadFile(updateConfigPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultNextcloudBaseURL
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.UpdatesFolder = strings.Trim(strings.TrimSpace(cfg.UpdatesFolder), "/")
	return cfg
}

func (c UpdateSourceConfig) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != ""
}
