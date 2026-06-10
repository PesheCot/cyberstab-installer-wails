package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"

	"cyberstab-installer/pkg/db"
	installer "cyberstab-installer/pkg/installer"
	"cyberstab-installer/pkg/system"
)

type InstallCallbacks struct {
	OnSteps    func(steps []installerStepDTO)
	OnProgress func(pct int, status string, steps []installerStepDTO)
	OnDeploy   func(pct int, status string)
}

type UninstallResult struct {
	Report   string
	Deferred bool
}

func validateInstallOptions(opts StartInstallOptions) error {
	if opts.SourceRoot == "" {
		return fmt.Errorf("source root directory is required")
	}
	opts.SourceRoot = filepath.Clean(opts.SourceRoot)
	if fi, err := os.Stat(opts.SourceRoot); err != nil {
		return fmt.Errorf("source root does not exist: %w", err)
	} else if !fi.IsDir() {
		return fmt.Errorf("source root is not a directory")
	}
	if !opts.InstallServer && !opts.InstallClients && !opts.InstallDB {
		return fmt.Errorf("at least one component must be selected for installation")
	}
	if strings.TrimSpace(opts.DbAction) == "restore" && strings.TrimSpace(opts.RestoreSqlPath) == "" {
		return fmt.Errorf("укажите путь к файлу .sql для восстановления базы")
	}
	return nil
}

func (a *App) runInstall(opts StartInstallOptions, cb InstallCallbacks) error {
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

	if err := validateInstallOptions(opts); err != nil {
		clearRunning()
		return err
	}

	e := installer.NewEngine()
	a.mu.Lock()
	a.currentEngine = e
	a.mu.Unlock()

	e.Options.Components.InstallServer = opts.InstallServer
	e.Options.Components.InstallClients = opts.InstallClients
	e.Options.Components.InstallDB = opts.InstallDB
	if kind := strings.TrimSpace(strings.ToLower(opts.DBEngine)); kind != "" {
		_ = a.SelectDbEngine(kind)
	}
	e.Options.SourceRoot = filepath.Clean(opts.SourceRoot)
	e.Options.PostgresPassword = opts.PostgresPassword
	e.Options.ReinstallExisting = opts.ReinstallExisting
	e.Options.DBRestoreFile = strings.TrimSpace(opts.RestoreSqlPath)
	applyDBModeFromWizard(&e.Options, opts.DbAction)
	e.PgUser = strings.TrimSpace(opts.PostgresUser)
	e.PgPassword = opts.PostgresPassword
	e.UninstallerData = embeddedUninstallerBytes()
	if opts.InstallDir != "" {
		e.InstallDir = opts.InstallDir
	}
	e.ConfigureSteps()

	emitSteps := func() {
		if cb.OnSteps != nil {
			cb.OnSteps(serializeEngineSteps(e.Steps))
		}
	}

	e.ProgressEmitter = func(pct int, status string) {
		steps := serializeEngineSteps(e.Steps)
		if cb.OnProgress != nil {
			cb.OnProgress(pct, status, steps)
		}
		log.Printf("[INSTALL] %d%% - %s", pct, status)
	}
	e.DeployProgressEmitter = func(pct int, status string) {
		if cb.OnDeploy != nil {
			cb.OnDeploy(pct, status)
		}
		log.Printf("[DEPLOY] %d%% - %s", pct, status)
	}
	e.SetUpdateHandler(emitSteps)

	emitSteps()
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
	if a.currentEngine == e {
		a.currentEngine = nil
	}
	a.mu.Unlock()

	if runErr != nil {
		log.Printf("[INSTALL] Failed: %v", runErr)
		return runErr
	}
	log.Printf("[INSTALL] Completed successfully")
	return nil
}

func (a *App) runUninstallCore(opts UninstallOptions) (UninstallResult, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return UninstallResult{}, fmt.Errorf("install is currently running — cannot uninstall")
	}
	a.running = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	var reports []string

	if stdruntime.GOOS == "windows" || stdruntime.GOOS == "linux" {
		_ = system.StopServer(strings.TrimSpace(opts.InstallDir))
	}

	if !opts.SkipDB {
		if kind := strings.TrimSpace(strings.ToLower(opts.DBEngine)); kind != "" {
			_ = a.SelectDbEngine(kind)
		}
		ensurePgRunning()
		pg, pgErr := db.CheckPostgres()
		if pgErr != nil || pg == nil || !pg.Installed {
			log.Printf("[UNINSTALL] PostgreSQL tools not found, skipping DB cleanup: %v", pgErr)
			reports = append(reports, "PostgreSQL not found — DB cleanup skipped")
		} else {
			if err := db.DropOkidociDB(opts.PostgresUser, opts.PostgresPassword); err != nil {
				log.Printf("[UNINSTALL] DropOkidociDB warning: %v", err)
				reports = append(reports, fmt.Sprintf("okidoci_db/roles: %v", err))
			} else {
				reports = append(reports, "okidoci_db and okidoci_* roles: dropped")
			}
			if err := db.DropOkiUserRoles(opts.PostgresUser, opts.PostgresPassword); err != nil {
				log.Printf("[UNINSTALL] DropOkiUserRoles warning: %v", err)
				reports = append(reports, fmt.Sprintf("oki_* roles: %v", err))
			} else {
				reports = append(reports, "oki_* roles: dropped")
			}
		}
	} else {
		reports = append(reports, "БД: пропуск по запросу пользователя")
	}

	installDir := opts.InstallDir
	if installDir == "" {
		if stdruntime.GOOS == "linux" {
			installDir = system.DefaultInstallDir
		} else {
			candidates := []string{`C:\Program Files\Cyberstab`, `C:\cyberstab`}
			for _, c := range candidates {
				if st, err := os.Stat(c); err == nil && st.IsDir() {
					installDir = c
					break
				}
			}
			if installDir == "" {
				installDir = system.DefaultInstallDir
			}
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
	return UninstallResult{Report: reportStr, Deferred: deferred}, nil
}

func defaultInstallDir() string {
	if stdruntime.GOOS == "linux" {
		return "/opt/cyberstab"
	}
	return `C:\Program Files\Cyberstab`
}
