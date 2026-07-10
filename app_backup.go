package main

import (
	"strings"

	"cyberstab-installer/pkg/system"
)

type DatabaseBackupResultDTO struct {
	Success         bool   `json:"success"`
	Path            string `json:"path"`
	Message         string `json:"message"`
	ServerRestarted bool   `json:"serverRestarted"`
	Log             string `json:"log,omitempty"`
}

func (a *App) BackupServerDatabase(installDir string) (DatabaseBackupResultDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	res, err := system.RunDatabaseBackup(installDir)
	if err != nil {
		return DatabaseBackupResultDTO{
			Success: false,
			Path:    res.Path,
			Message: err.Error(),
			Log:     res.Message,
		}, err
	}
	return DatabaseBackupResultDTO{
		Success:         true,
		Path:            res.Path,
		Message:         res.Message,
		ServerRestarted: res.ServerRestarted,
		Log:             res.Message,
	}, nil
}

func (a *App) RevealPathInExplorer(path string) error {
	return system.RevealPathInExplorer(path)
}
