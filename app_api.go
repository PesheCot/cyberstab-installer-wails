package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"cyberstab-installer/pkg/db"
	"cyberstab-installer/pkg/fs"
	installer "cyberstab-installer/pkg/installer"
)

// AppInfo is returned to the frontend for OS and environment display.
type AppInfo struct {
	OS                string `json:"os"`
	NeedAdmin         bool   `json:"needAdmin"`
	PostgresInstalled bool   `json:"postgresInstalled"`
}

// GetAppInfo reports OS, elevation, and whether DB tools were detected.
func (a *App) GetAppInfo() AppInfo {
	e := installer.NewEngine()
	osName := "windows"
	switch {
	case runtime.GOOS == "linux":
		osName = "linux"
	case runtime.GOOS != "windows":
		osName = runtime.GOOS
	}
	engines, _ := db.DiscoverEngines()
	installed := len(engines) > 0
	return AppInfo{
		OS:                osName,
		NeedAdmin:         e.NeedSudo(),
		PostgresInstalled: installed,
	}
}

// PreviewInstallDone is used only to open the desktop app on the final install screen for UI review.
func (a *App) PreviewInstallDone() bool {
	if os.Getenv("CYBERSTAB_PREVIEW_INSTALL_DONE") == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "--preview-install-done" {
			return true
		}
	}
	return false
}

type DbEngineDTO struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	BinDir      string `json:"binDir"`
	Version     string `json:"version"`
	IsManual    bool   `json:"isManual"`
}

// DbCheckResult is used by the DB engine step in the wizard.
type DbCheckResult struct {
	Engines             []DbEngineDTO `json:"engines"`
	Installed           bool          `json:"installed"`
	InstallerFound      bool          `json:"installerFound"`
	InstallerPath       string        `json:"installerPath"`
	ActiveEngineKind    string        `json:"activeEngineKind"`
}

// CheckDbInstalled reports discovered DB engines and whether a PostgreSQL installer exists on media.
func (a *App) CheckDbInstalled() (DbCheckResult, error) {
	engines, err := db.DiscoverEngines()
	if err != nil {
		return DbCheckResult{}, err
	}
	dto := make([]DbEngineDTO, 0, len(engines))
	for _, e := range engines {
		isManual := false
		if active := db.GetActiveEngine(); strings.TrimSpace(active.BinDir) != "" {
			isManual = strings.EqualFold(filepath.Clean(active.BinDir), filepath.Clean(e.BinDir)) &&
				!strings.Contains(strings.ToLower(strings.ReplaceAll(e.BinDir, "/", `\`)), `\postgresql\`) &&
				!strings.Contains(strings.ToLower(strings.ReplaceAll(e.BinDir, "/", `\`)), `\gis\jatoba\`)
		}
		dto = append(dto, DbEngineDTO{
			Kind:     string(e.Kind),
			Label:    e.DisplayName,
			BinDir:   e.BinDir,
			Version:  e.Version,
			IsManual: isManual,
		})
	}
	path := ""
	if len(dto) == 0 {
		path = findPostgresInstallerOnDrives()
	}
	active := db.GetActiveEngine()
	return DbCheckResult{
		Engines:          dto,
		Installed:        len(dto) > 0,
		InstallerFound:   path != "",
		InstallerPath:    path,
		ActiveEngineKind: string(active.Kind),
	}, nil
}

// OkidociCheckResult reports whether okidoci_db exists.
type OkidociCheckResult struct {
	Exists bool `json:"exists"`
}

// CheckOkidociDB checks whether database okidoci_db exists.
func (a *App) CheckOkidociDB(user, password string) (OkidociCheckResult, error) {
	if strings.TrimSpace(password) == "" {
		return OkidociCheckResult{}, fmt.Errorf("нужен пароль PostgreSQL")
	}
	ok, err := db.OkidociDatabaseExists(user, password)
	if err != nil {
		return OkidociCheckResult{}, err
	}
	return OkidociCheckResult{Exists: ok}, nil
}

// InstallDirConflict is returned when the target install directory may need user confirmation.
type InstallDirConflict struct {
	Exists bool `json:"exists"`
}

// CheckInstallDirConflict returns whether the install directory already exists.
func (a *App) CheckInstallDirConflict(dir string) (InstallDirConflict, error) {
	d := filepath.Clean(strings.TrimSpace(dir))
	if d == "" || d == "." {
		return InstallDirConflict{Exists: false}, nil
	}
	if st, err := os.Stat(d); err == nil && st.IsDir() {
		return InstallDirConflict{Exists: true}, nil
	}
	return InstallDirConflict{Exists: false}, nil
}

// AutoDetectSourceRoot searches removable drives and fixed volumes for a Cyberstab distro parent folder.
func (a *App) AutoDetectSourceRoot(wantServerOrDB, wantClients bool) (string, error) {
	f := fs.NewFinder()
	serverDirName := "CyberstabServerLinux"
	if isWindows() {
		serverDirName = "CyberstabServerWindows"
	}
	var targets []string
	if wantServerOrDB {
		targets = append(targets, serverDirName)
	}
	if wantClients {
		if isWindows() {
			targets = append(targets, "CyberstabClientWindows32", "CyberstabClientWindows64")
		} else {
			targets = append(targets, "CyberstabClientLinux32", "CyberstabClientLinux64")
		}
	}
	if len(targets) == 0 {
		targets = f.ServerDirNames
	}
	f.ServerDirNames = targets
	found := f.FindDistros()
	if len(found) == 0 {
		return "", nil
	}
	return found[0], nil
}

// PickFolder opens a native folder picker dialog.
func (a *App) PickFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите папку",
	})
}

// PickDbDir lets user choose a DB engine root (must contain bin\psql).
func (a *App) PickDbDir() (string, error) {
	p, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите папку установки СУБД (PostgreSQL/Jatoba)",
	})
	if err != nil || strings.TrimSpace(p) == "" {
		return "", err
	}
	bin := filepath.Join(p, "bin")
	psql := filepath.Join(bin, "psql.exe")
	if !isWindows() {
		psql = filepath.Join(bin, "psql")
	}
	if _, err := os.Stat(psql); err != nil {
		return "", fmt.Errorf("не найден %s", psql)
	}
	db.SetPostgresBinDir(bin)
	return p, nil
}

// SelectDbEngineBin activates a discovered DB engine by bin directory (precise when several are installed).
func (a *App) SelectDbEngineBin(binDir string) error {
	_, err := db.SelectEngineByBinDir(binDir)
	return err
}

// SelectDbEngine activates one of discovered DB engines by kind.
func (a *App) SelectDbEngine(kind string) error {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case string(db.EnginePostgreSQL):
		_, err := db.SelectEngineByKind(db.EnginePostgreSQL)
		return err
	case string(db.EnginePostgresPro):
		_, err := db.SelectEngineByKind(db.EnginePostgresPro)
		return err
	case string(db.EngineJatoba):
		_, err := db.SelectEngineByKind(db.EngineJatoba)
		return err
	default:
		return fmt.Errorf("неизвестный тип СУБД: %s", kind)
	}
}

// PickSqlFile opens a file picker for .sql backups.
func (a *App) PickSqlFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите файл резервной копии SQL",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL (*.sql)", Pattern: "*.sql"},
		},
	})
}

// VerifyPostgresPassword checks PostgreSQL credentials and superuser privileges required for install.
func (a *App) VerifyPostgresPassword(user, password string) error {
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("нужен пароль СУБД")
	}
	return db.VerifyPostgresCredentials(user, password)
}

// ResetPostgresPassword sets a new password for the given PostgreSQL role (connects as postgres via local trust).
func (a *App) ResetPostgresPassword(username, newPassword string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("укажите пользователя PostgreSQL")
	}
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("новый пароль не должен быть пустым")
	}
	return db.SetUserPassword(username, newPassword)
}

// InstallPostgresFromUsb runs the PostgreSQL installer found under the detected distro root (same search as AutoDetectSourceRoot).
func (a *App) InstallPostgresFromUsb() error {
	f := fs.NewFinder()
	roots := f.FindDistros()
	if len(roots) == 0 {
		return fmt.Errorf("не найдена папка дистрибутива (нужен USB/путь с CyberstabServer*)")
	}
	return installer.InstallPostgresFromSource(roots[0])
}

func findPostgresInstallerOnDrives() string {
	f := fs.NewFinder()
	for _, root := range f.FindDistros() {
		if p := findPostgresInstallerUnder(root); p != "" {
			return p
		}
	}
	if isWindows() {
		for c := byte('A'); c <= 'Z'; c++ {
			root := fmt.Sprintf("%c:\\", c)
			if _, err := os.Stat(root); err != nil {
				continue
			}
			if p := findPostgresInstallerUnder(root); p != "" {
				return p
			}
		}
		return ""
	}
	for _, root := range []string{"/mnt", "/media", "/run/media"} {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		if p := findPostgresInstallerUnder(root); p != "" {
			return p
		}
	}
	return ""
}

func findPostgresInstallerUnder(root string) string {
	var hit string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(root, path)
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(info.Name())
		if (strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".msi")) && strings.Contains(name, "postgres") {
			if strings.Contains(name, "setup") || strings.Contains(name, "install") || strings.HasPrefix(name, "postgresql") {
				hit = path
				return filepath.SkipAll
			}
			if hit == "" {
				hit = path
			}
		}
		return nil
	})
	return hit
}

// LaunchClient opens the Cyberstab client executable.
func (a *App) LaunchClient(installDir string) error {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		if isWindows() {
			installDir = `C:\Program Files\Cyberstab`
		} else {
			installDir = "/opt/cyberstab"
		}
	}
	clientDir := installer.DetectClientDir(installDir)
	if clientDir == "" {
		return fmt.Errorf("client directory not found")
	}

	clientExe := installer.FindClientExeBestEffort(clientDir)
	if clientExe == "" {
		return fmt.Errorf("client executable not found")
	}

	cmd := exec.Command(clientExe)
	cmd.Dir = filepath.Dir(clientExe)
	hideCmdWindow(cmd, true)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	return nil
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func applyDBModeFromWizard(o *installer.InstallOptions, action string) {
	switch strings.TrimSpace(action) {
	case "skip":
		o.DBMode = installer.DBModeKeep
	case "restore":
		o.DBMode = installer.DBModeRestore
	case "new":
		o.DBMode = installer.DBModeRecreate
	default:
		o.DBMode = installer.DBModeAuto
	}
}

type installerStepDTO struct {
	Index       int    `json:"Index"`
	Description string `json:"Description"`
	Status      string `json:"Status"`
	IsActive    bool   `json:"IsActive"`
	IsDone      bool   `json:"IsDone"`
	ErrorText   string `json:"Error,omitempty"`
}

func serializeEngineSteps(steps []installer.StepInfo) []installerStepDTO {
	out := make([]installerStepDTO, 0, len(steps))
	for _, s := range steps {
		d := installerStepDTO{
			Index: s.Index, Description: s.Description, Status: s.Status,
			IsActive: s.IsActive, IsDone: s.IsDone,
		}
		if s.ErrorText != "" {
			d.ErrorText = s.ErrorText
		} else if s.Error != nil {
			d.ErrorText = s.Error.Error()
		}
		out = append(out, d)
	}
	return out
}
