package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"cyberstab-installer/pkg/db"
	installer "cyberstab-installer/pkg/installer"
	"cyberstab-installer/pkg/system"
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	running bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// DomReady is called after the front-end dom has been loaded
func (a *App) DomReady(ctx context.Context) {
	// Add your action here
}

// BeforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will cause it to shut down.
func (a *App) BeforeClose(ctx context.Context) bool {
	return true
}

// Shutdown is called at application termination
func (a *App) Shutdown(ctx context.Context) {
	// Perform your tear-down operations here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

type ServerStatusDTO struct {
	TaskExists bool   `json:"taskExists"`
	Running    bool   `json:"running"`
	Raw        string `json:"raw,omitempty"`
}

type ServerInfoDTO struct {
	Status       ServerStatusDTO `json:"status"`
	Connections  string          `json:"connections,omitempty"`
	Version      string          `json:"version,omitempty"`
	SessionCount int             `json:"sessionCount"` // без omitempty: 0 — валидное значение
	ConsoleErr   string          `json:"consoleErr,omitempty"`
	RawConsole   string          `json:"rawConsole,omitempty"`
}

func (a *App) GetServerStatus(installDir string) (ServerStatusDTO, error) {
	st, err := system.QueryServerStatus()
	return ServerStatusDTO{TaskExists: st.TaskExists, Running: st.Running, Raw: st.Raw}, err
}

func (a *App) GetServerInfo(installDir string, pgPassword string) (ServerInfoDTO, error) {
	st, err := a.GetServerStatus(installDir)
	info := ServerInfoDTO{Status: st}
	if err != nil {
		return info, err
	}
	// Only query console when the autostart task exists (best-effort).
	if st.TaskExists {
		if strings.TrimSpace(installDir) == "" {
			installDir = system.DefaultInstallDir
		}
		c, cerr := system.QueryServerConsoleInfo(installDir, pgPassword)
		info.Connections = strings.TrimSpace(c.ConnectionsText)
		info.Version = strings.TrimSpace(c.VersionText)
		info.SessionCount = c.SessionCount
		info.RawConsole = c.RawOutput
		if cerr != nil {
			info.ConsoleErr = cerr.Error()
		}
	}
	return info, nil
}

func (a *App) StartServer() error {
	return system.StartServer()
}

func (a *App) StopServer(installDir string) error {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	return system.StopServer(installDir)
}

func (a *App) RestartServer(installDir string) error {
	if err := a.StopServer(installDir); err != nil {
		return err
	}
	time.Sleep(800 * time.Millisecond)
	return a.StartServer()
}

// StartInstallOptions contains all options needed to start an installation.
type StartInstallOptions struct {
	InstallServer     bool   `json:"installServer"`
	InstallClients    bool   `json:"installClients"`
	InstallDB         bool   `json:"installDB"`
	SourceRoot        string `json:"sourceRoot"`
	PostgresPassword  string `json:"postgresPassword"`
	InstallDir        string `json:"installDir"`
	DbAction          string `json:"dbAction"` // "skip" | "new" | "restore" | ""
	RestoreSqlPath    string `json:"restoreSqlPath"`
	ReinstallExisting bool   `json:"reinstallExisting"`
}

// UninstallOptions contains parameters for the uninstall operation.
type UninstallOptions struct {
	InstallDir       string `json:"installDir"`
	PostgresPassword string `json:"postgresPassword"`
	SkipDB           bool   `json:"skipDB"`
}

// StartInstall begins the installation process.
// It validates input, prepares the installer engine, and runs the installation steps.
func (a *App) StartInstall(opts StartInstallOptions) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("install is currently running")
	}
	a.running = true
	a.mu.Unlock()

	clearRunning := func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}

	// Validate required options
	if opts.SourceRoot == "" {
		clearRunning()
		return fmt.Errorf("source root directory is required")
	}

	// Normalize source root path
	opts.SourceRoot = filepath.Clean(opts.SourceRoot)
	if fi, err := os.Stat(opts.SourceRoot); err != nil {
		clearRunning()
		return fmt.Errorf("source root does not exist: %w", err)
	} else if !fi.IsDir() {
		clearRunning()
		return fmt.Errorf("source root is not a directory")
	}

	// Validate that at least one component is selected
	if !opts.InstallServer && !opts.InstallClients && !opts.InstallDB {
		clearRunning()
		return fmt.Errorf("at least one component must be selected for installation")
	}
	if strings.TrimSpace(opts.DbAction) == "restore" && strings.TrimSpace(opts.RestoreSqlPath) == "" {
		clearRunning()
		return fmt.Errorf("укажите путь к файлу .sql для восстановления базы")
	}

	// Initialize installer engine
	e := installer.NewEngine()
	e.Options.Components.InstallServer = opts.InstallServer
	e.Options.Components.InstallClients = opts.InstallClients
	e.Options.Components.InstallDB = opts.InstallDB
	e.Options.SourceRoot = opts.SourceRoot
	e.Options.PostgresPassword = opts.PostgresPassword
	e.Options.ReinstallExisting = opts.ReinstallExisting
	e.Options.DBRestoreFile = strings.TrimSpace(opts.RestoreSqlPath)
	applyDBModeFromWizard(&e.Options, opts.DbAction)
	e.PgPassword = opts.PostgresPassword
	e.UninstallerData = embeddedUninstallerBytes()
	if opts.InstallDir != "" {
		e.InstallDir = opts.InstallDir
	}

	emitSteps := func() {
		wailsruntime.EventsEmit(a.ctx, "install:step", map[string]interface{}{
			"steps": serializeEngineSteps(e.Steps),
		})
	}

	// Setup progress reporting
	e.ProgressEmitter = func(pct int, status string) {
		wailsruntime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"percentage": pct,
			"status":     status,
			"steps":      serializeEngineSteps(e.Steps),
		})
		wailsruntime.EventsEmit(a.ctx, "init:progress", map[string]interface{}{
			"percent": pct,
			"status":  status,
		})
		log.Printf("[INSTALL] %d%% - %s", pct, status)
	}

	e.DeployProgressEmitter = func(pct int, status string) {
		wailsruntime.EventsEmit(a.ctx, "deploy:progress", map[string]interface{}{
			"percentage": pct,
			"percent":    pct,
			"status":     status,
		})
		log.Printf("[DEPLOY] %d%% - %s", pct, status)
	}

	e.SetUpdateHandler(func() {
		emitSteps()
	})

	e.Run()
	<-e.Done()

	var runErr error
	for i := range e.Steps {
		if e.Steps[i].Error != nil {
			runErr = e.Steps[i].Error
			break
		}
	}

	a.mu.Lock()
	a.running = false
	a.mu.Unlock()

	if runErr != nil {
		log.Printf("[INSTALL] Failed: %v", runErr)
		wailsruntime.EventsEmit(a.ctx, "install:error", map[string]interface{}{
			"message": runErr.Error(),
		})
		return runErr
	}

	wailsruntime.EventsEmit(a.ctx, "install:done", map[string]interface{}{
		"message": "Installation completed successfully",
	})
	log.Printf("[INSTALL] Completed successfully")
	return nil
}

// Uninstall performs a full uninstall:
//   - Drops the okidoci_db database and all Cyberstab roles (okidoci_*, oki_*)
//   - Removes the Cyberstab installation directory
//   - Removes Windows scheduled tasks and desktop shortcuts
// PostgreSQL itself is NOT removed.
func (a *App) Uninstall(opts UninstallOptions) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("install is currently running — cannot uninstall")
	}
	a.running = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	var reports []string

	// 1. Drop database and roles via PostgreSQL (if Postgres is reachable).
	if !opts.SkipDB {
		if _, err := findPgDir(); err == nil {
			ensurePgRunning()

			// Сначала БД и основные роли okidoci_* (без DELETE sec_user — см. DropOkidociDB).
			if err := db.DropOkidociDB(opts.PostgresPassword); err != nil {
				log.Printf("[UNINSTALL] DropOkidociDB warning: %v", err)
				reports = append(reports, fmt.Sprintf("okidoci_db/roles: %v", err))
			} else {
				reports = append(reports, "okidoci_db and okidoci_* roles: dropped")
			}

			// Оставшиеся динамические oki_* на кластере.
			if err := db.DropOkiUserRoles(opts.PostgresPassword); err != nil {
				log.Printf("[UNINSTALL] DropOkiUserRoles warning: %v", err)
				reports = append(reports, fmt.Sprintf("oki_* roles: %v", err))
			} else {
				reports = append(reports, "oki_* roles: dropped")
			}
		} else {
			log.Printf("[UNINSTALL] PostgreSQL directory not found, skipping DB cleanup")
			reports = append(reports, "PostgreSQL not found — DB cleanup skipped")
		}
	} else {
		reports = append(reports, "БД: пропуск по запросу пользователя")
	}

	// 2. Remove installation directory, scheduled tasks, and shortcuts.
	installDir := opts.InstallDir
	if installDir == "" {
		// Fallback to default paths.
		candidates := []string{
			`C:\Program Files\Cyberstab`,
			`C:\cyberstab`,
		}
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && st.IsDir() {
				installDir = c
				break
			}
		}
		if installDir == "" {
			installDir = `C:\Program Files\Cyberstab`
		}
	}

	deferred, fsErr := system.UninstallCyberstab(installDir)
	if fsErr != nil {
		log.Printf("[UNINSTALL] RemoveCyberstab warning: %v", fsErr)
		reports = append(reports, fmt.Sprintf("files/tasks: %v", fsErr))
	} else if deferred {
		reports = append(reports, fmt.Sprintf("папка %s будет удалена после закрытия деинсталлятора", installDir))
	} else {
		reports = append(reports, fmt.Sprintf("install dir (%s) and tasks: removed", installDir))
	}

	reportStr := strings.Join(reports, " | ")
	log.Printf("[UNINSTALL] Complete: %s", reportStr)
	wailsruntime.EventsEmit(a.ctx, "uninstall:done", map[string]interface{}{
		"report":    reportStr,
		"success":   true,
		"deferred":  deferred,
	})
	if deferred {
		go func() {
			time.Sleep(450 * time.Millisecond)
			wailsruntime.Quit(a.ctx)
		}()
	}
	return nil
}

// ensurePgRunning ensures that PostgreSQL service is running.
// On Windows, it attempts to start the service if it's stopped.
func ensurePgRunning() {
	if stdruntime.GOOS != "windows" {
		return
	}
	db.StartPostgresServiceBestEffort()
}

// findPgDir finds the PostgreSQL installation directory.
func findPgDir() (string, error) {
	candidates := []string{
		os.Getenv("ProgramFiles") + "\\PostgreSQL",
		os.Getenv("ProgramFiles(x86)") + "\\PostgreSQL",
		`C:\Program Files\PostgreSQL`,
	}
	for _, base := range candidates {
		if base == "" || base == "PostgreSQL" {
			continue
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			psql := filepath.Join(base, e.Name(), "bin", "psql.exe")
			if _, err := os.Stat(psql); err == nil {
				return filepath.Join(base, e.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("PostgreSQL not found")
}
