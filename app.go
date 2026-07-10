package main

import (
	"context"
	"fmt"
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
	ctx           context.Context
	mu            sync.Mutex
	running       bool
	currentEngine *installer.Engine
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
	_ = a.CancelInstall()
	return true
}

// Shutdown is called at application termination
func (a *App) Shutdown(ctx context.Context) {
	_ = a.CancelInstall()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) CancelInstall() error {
	a.mu.Lock()
	engine := a.currentEngine
	a.running = false
	a.currentEngine = nil
	a.mu.Unlock()

	if engine != nil {
		engine.Cancel()
	} else {
		installer.CancelActiveInstallerProcesses()
	}
	return nil
}

type ServerStatusDTO struct {
	TaskExists     bool   `json:"taskExists"`
	Running        bool   `json:"running"`
	Raw            string `json:"raw,omitempty"`
	NetworkPort    int    `json:"networkPort"`
	ManagementPort int    `json:"managementPort"`
	PropertiesPath string `json:"propertiesPath,omitempty"`
}

type ServerSessionDTO struct {
	UserID   int    `json:"userId"`
	Login    string `json:"login"`
	Username string `json:"username"`
	IP       string `json:"ip"`
	Company  string `json:"company"`
	Module   string `json:"module"`
}

type ServerInfoDTO struct {
	Status       ServerStatusDTO    `json:"status"`
	Sessions     []ServerSessionDTO `json:"sessions"`
	Version      string             `json:"version,omitempty"`
	SessionCount int                `json:"sessionCount"`
}

func (a *App) GetServerStatus(installDir string) (ServerStatusDTO, error) {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	st, err := system.QueryServerStatus(installDir)
	return ServerStatusDTO{
		TaskExists:     st.TaskExists,
		Running:        st.Running,
		Raw:            st.Raw,
		NetworkPort:    st.NetworkPort,
		ManagementPort: st.ManagementPort,
		PropertiesPath: st.PropertiesPath,
	}, err
}

func (a *App) GetServerInfo(installDir string, pgPassword string) (ServerInfoDTO, error) {
	_ = pgPassword
	st, err := a.GetServerStatus(installDir)
	info := ServerInfoDTO{
		Status:  st,
		Version: system.ReadInstalledServerVersion(installDir),
	}
	if st.Running {
		if live, liveErr := system.QueryServerLiveInfo(installDir); liveErr == nil {
			info.SessionCount = live.SessionCount()
			info.Sessions = mapServerSessions(live.Sessions)
		}
	}
	return info, err
}

func mapServerSessions(in []system.ServerSession) []ServerSessionDTO {
	out := make([]ServerSessionDTO, 0, len(in))
	for _, s := range in {
		out = append(out, ServerSessionDTO{
			UserID:   s.UserID,
			Login:    s.Login,
			Username: s.Username,
			IP:       s.IP,
			Company:  s.Company,
			Module:   s.Module,
		})
	}
	return out
}

func (a *App) DisconnectServerUser(installDir string, userID int) error {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	return system.DisconnectServerUser(installDir, userID)
}

func (a *App) DisconnectAllServerUsers(installDir string) error {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	return system.DisconnectAllServerUsers(installDir)
}

func (a *App) CheckInternet() bool {
	return system.HasInternetAccess()
}

func (a *App) StartServer(installDir string) error {
	if strings.TrimSpace(installDir) == "" {
		installDir = system.DefaultInstallDir
	}
	return system.StartServer(installDir)
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
	return a.StartServer(installDir)
}

// StartInstallOptions contains all options needed to start an installation.
type StartInstallOptions struct {
	InstallServer     bool   `json:"installServer"`
	InstallClients    bool   `json:"installClients"`
	InstallDB         bool   `json:"installDB"`
	SourceRoot        string `json:"sourceRoot"`
	DBEngine          string `json:"dbEngine"`
	PostgresUser      string `json:"postgresUser"`
	PostgresPassword  string `json:"postgresPassword"`
	InstallDir        string `json:"installDir"`
	DbAction          string `json:"dbAction"` // "skip" | "new" | "restore" | ""
	RestoreSqlPath    string `json:"restoreSqlPath"`
	ReinstallExisting bool   `json:"reinstallExisting"`
}

// UninstallOptions contains parameters for the uninstall operation.
type UninstallOptions struct {
	InstallDir       string `json:"installDir"`
	DBEngine         string `json:"dbEngine"`
	PostgresUser     string `json:"postgresUser"`
	PostgresPassword string `json:"postgresPassword"`
	SkipDB           bool   `json:"skipDB"`
}

// StartInstall begins the installation process.
func (a *App) StartInstall(opts StartInstallOptions) error {
	cb := InstallCallbacks{
		OnSteps: func(steps []installerStepDTO) {
			wailsruntime.EventsEmit(a.ctx, "install:step", map[string]interface{}{
				"steps": steps,
			})
		},
		OnProgress: func(pct int, status string, steps []installerStepDTO) {
			wailsruntime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
				"percentage": pct,
				"status":     status,
				"steps":      steps,
			})
			wailsruntime.EventsEmit(a.ctx, "init:progress", map[string]interface{}{
				"percent": pct,
				"status":  status,
			})
		},
		OnDeploy: func(pct int, status string) {
			wailsruntime.EventsEmit(a.ctx, "deploy:progress", map[string]interface{}{
				"percentage": pct,
				"percent":    pct,
				"status":     status,
			})
		},
	}

	if err := a.runInstall(opts, cb); err != nil {
		wailsruntime.EventsEmit(a.ctx, "install:error", map[string]interface{}{
			"message": err.Error(),
		})
		return err
	}

	wailsruntime.EventsEmit(a.ctx, "install:done", map[string]interface{}{
		"message": "Installation completed successfully",
	})
	return nil
}

// Uninstall performs a full uninstall (PostgreSQL itself is NOT removed).
func (a *App) Uninstall(opts UninstallOptions) error {
	res, err := a.runUninstallCore(opts)
	if err != nil {
		return err
	}
	wailsruntime.EventsEmit(a.ctx, "uninstall:done", map[string]interface{}{
		"report":   res.Report,
		"success":  true,
		"deferred": res.Deferred,
	})
	if res.Deferred {
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
	if stdruntime.GOOS == "windows" || stdruntime.GOOS == "linux" {
		db.StartPostgresServiceBestEffort()
	}
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
