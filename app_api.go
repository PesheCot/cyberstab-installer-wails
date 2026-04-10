package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

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

// GetAppInfo reports OS, elevation, and whether PostgreSQL tools were detected.
func (a *App) GetAppInfo() AppInfo {
	e := installer.NewEngine()
	osName := "windows"
	switch {
	case runtime.GOOS == "linux":
		osName = "linux"
	case runtime.GOOS != "windows":
		osName = runtime.GOOS
	}
	pg, _ := db.CheckPostgres()
	installed := pg != nil && pg.Installed
	return AppInfo{
		OS:                osName,
		NeedAdmin:         e.NeedSudo(),
		PostgresInstalled: installed,
	}
}

// PgCheckResult is used by the PostgreSQL step in the wizard.
type PgCheckResult struct {
	Installed      bool   `json:"installed"`
	InstallerFound bool   `json:"installerFound"`
	InstallerPath  string `json:"installerPath"`
}

// CheckPgInstalled reports whether PostgreSQL is already installed and whether an installer was found on media.
func (a *App) CheckPgInstalled() (PgCheckResult, error) {
	pg, err := db.CheckPostgres()
	if err != nil {
		return PgCheckResult{}, err
	}
	installed := pg != nil && pg.Installed
	path := ""
	if !installed {
		path = findPostgresInstallerOnDrives()
	}
	return PgCheckResult{
		Installed:      installed,
		InstallerFound: path != "",
		InstallerPath:  path,
	}, nil
}

// OkidociCheckResult reports whether okidoci_db exists.
type OkidociCheckResult struct {
	Exists bool `json:"exists"`
}

// CheckOkidociDB checks whether database okidoci_db exists.
func (a *App) CheckOkidociDB(password string) (OkidociCheckResult, error) {
	if strings.TrimSpace(password) == "" {
		return OkidociCheckResult{}, fmt.Errorf("нужен пароль postgres")
	}
	ok, err := db.OkidociDatabaseExists(password)
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

// PickPgDir lets the user choose a PostgreSQL installation directory (must contain bin\psql).
func (a *App) PickPgDir() (string, error) {
	p, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите папку установки PostgreSQL",
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

// PickSqlFile opens a file picker for .sql backups.
func (a *App) PickSqlFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Выберите файл резервной копии SQL",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL (*.sql)", Pattern: "*.sql"},
		},
	})
}

// VerifyPostgresPassword checks the postgres password (quick check).
func (a *App) VerifyPostgresPassword(password string) error {
	return db.VerifyPassword(password)
}

// ResetPostgresPassword sets a new password for postgres (uses trust injection on Windows when needed).
func (a *App) ResetPostgresPassword(newPassword string) error {
	return db.SetUserPassword("postgres", newPassword)
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
func (a *App) LaunchClient() error {
	installDir := `C:\Program Files\Cyberstab`
	// Detect client dir based on architecture
	clientDir := installer.DetectClientDirWindows(installDir)
	if clientDir == "" {
		return fmt.Errorf("client directory not found")
	}
	
	// Find client exe
	clientExe := installer.FindClientExeBestEffort(clientDir)
	if clientExe == "" {
		return fmt.Errorf("client executable not found")
	}
	
	// Launch client with proper working directory
	cmd := exec.Command(clientExe)
	cmd.Dir = filepath.Dir(clientExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: false, // Show the window for client
	}
	
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
