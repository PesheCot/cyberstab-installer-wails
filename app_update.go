package main

import (
	"fmt"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"cyberstab-installer/pkg/system"
)

type UpdateSourceConfigDTO struct {
	BaseURL       string `json:"baseURL"`
	UpdatesFolder string `json:"updatesFolder"`
	ConfigPath    string `json:"configPath"`
	Configured    bool   `json:"configured"`
}

type ServerUpdateTargetDTO struct {
	Platform    string `json:"platform"`
	ArchiveName string `json:"archiveName"`
}

type NextcloudUpdateCheckDTO struct {
	CurrentVersion string `json:"currentVersion"`
	RemoteVersion  string `json:"remoteVersion"`
	UpdateRequired bool   `json:"updateRequired"`
	ArchiveName    string `json:"archiveName"`
	Message        string `json:"message"`
}

type ServerUpdateResultDTO struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ArchivePath string `json:"archivePath,omitempty"`
	Log         string `json:"log,omitempty"`
}

func (a *App) GetUpdateSourceConfig() UpdateSourceConfigDTO {
	cfg := system.LoadUpdateSourceConfig()
	return UpdateSourceConfigDTO{
		BaseURL:       cfg.BaseURL,
		UpdatesFolder: cfg.UpdatesFolder,
		ConfigPath:    system.UpdateConfigPath(),
		Configured:    cfg.Configured(),
	}
}

func (a *App) GetServerUpdateTarget(installDir string) ServerUpdateTargetDTO {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	platform := system.InstalledServerPlatform(installDir)
	return ServerUpdateTargetDTO{
		Platform:    platform,
		ArchiveName: system.ServerUpdateArchiveName(installDir),
	}
}

func (a *App) GetNextcloudSavedLogin() string {
	login, ok := system.NextcloudSavedLogin()
	if !ok {
		return ""
	}
	return login
}

func (a *App) SaveNextcloudCredentials(login, password string, remember bool) error {
	return system.SaveNextcloudCredentials(login, password, remember)
}

func (a *App) CheckNextcloudServerUpdate(installDir, login, password string) (NextcloudUpdateCheckDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	check, err := system.CheckNextcloudServerUpdate(installDir, login, password)
	if err != nil {
		return NextcloudUpdateCheckDTO{}, err
	}
	return mapNextcloudUpdateCheck(check), nil
}

func (a *App) CheckNextcloudServerUpdateWithSaved(installDir string) (NextcloudUpdateCheckDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	login, password, ok := system.LoadNextcloudCredentials()
	if !ok {
		return NextcloudUpdateCheckDTO{}, fmt.Errorf("сохранённые учётные данные не найдены")
	}
	check, err := system.CheckNextcloudServerUpdate(installDir, login, password)
	if err != nil {
		return NextcloudUpdateCheckDTO{}, err
	}
	return mapNextcloudUpdateCheck(check), nil
}

func mapNextcloudUpdateCheck(check system.NextcloudUpdateCheck) NextcloudUpdateCheckDTO {
	return NextcloudUpdateCheckDTO{
		CurrentVersion: check.CurrentVersion,
		RemoteVersion:  check.RemoteVersion,
		UpdateRequired: check.UpdateRequired,
		ArchiveName:    check.ArchiveName,
		Message:        check.Message,
	}
}

func (a *App) RunServerUpdateFromNextcloud(installDir, login, password string, remember, force bool) (ServerUpdateResultDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	msg, archivePath, err := system.RunServerUpdateFromNextcloud(installDir, login, password, remember, force)
	if err != nil {
		return ServerUpdateResultDTO{
			Success:     false,
			Message:     err.Error(),
			ArchivePath: archivePath,
			Log:         msg,
		}, err
	}
	return ServerUpdateResultDTO{
		Success:     true,
		Message:     msg,
		ArchivePath: archivePath,
		Log:         msg,
	}, nil
}

func (a *App) RunServerUpdateFromNextcloudSaved(installDir string, force bool) (ServerUpdateResultDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	login, password, ok := system.LoadNextcloudCredentials()
	if !ok {
		return ServerUpdateResultDTO{}, fmt.Errorf("сохранённые учётные данные не найдены")
	}
	return a.RunServerUpdateFromNextcloud(installDir, login, password, true, force)
}

func (a *App) RunServerUpdateFromPath(installDir, archivePath string, force bool) (ServerUpdateResultDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	msg, err := system.RunServerUpdateFromLocalPath(installDir, archivePath, force)
	if err != nil {
		return ServerUpdateResultDTO{
			Success: false,
			Message: err.Error(),
			Log:     msg,
		}, err
	}
	return ServerUpdateResultDTO{
		Success: true,
		Message: msg,
		Log:     msg,
	}, nil
}

func (a *App) PickUpdateArchive() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите архив обновления",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Архивы Cyberstab (*.zip)", Pattern: "CyberstabServer*.zip;*.zip;*.tar.gz"},
		},
	})
}

func (a *App) PickUpdateFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите папку с архивом обновления",
	})
}
