package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cyberstab-installer/pkg/db"
)

type ComponentsOptions struct {
	InstallServer  bool
	InstallClients bool
	InstallDB      bool
}

type DBMode string

const (
	DBModeAuto     DBMode = "auto"
	DBModeKeep     DBMode = "keep"
	DBModeRecreate DBMode = "recreate"
	DBModeRestore  DBMode = "restore"
)

type InstallOptions struct {
	Components        ComponentsOptions
	SourceRoot        string
	PostgresPassword  string
	ReinstallExisting bool
	DBRestoreFile     string
	DBMode            DBMode
}

type StepInfo struct {
	Index       int
	Description string
	Status      string
	IsActive    bool
	IsDone      bool
	Error       error
	ErrorText   string
}

type Engine struct {
	Options         InstallOptions
	Steps           []StepInfo
	DoneCh          chan struct{}
	CancelCh        chan struct{}
	PgUser          string
	PgPassword      string
	InstallDir      string
	UninstallerData []byte

	ProgressEmitter       func(pct int, status string)
	DeployProgressEmitter func(pct int, status string)

	mu           sync.Mutex
	update       func()
	doneOnce     sync.Once
	cancelOnce   sync.Once
	runningError error
}

func NewEngine() *Engine {
	return &Engine{
		DoneCh:   make(chan struct{}),
		CancelCh: make(chan struct{}),
	}
}

// ConfigureSteps builds the visible install step list from selected options.
func (e *Engine) ConfigureSteps() {
	e.Steps = buildInstallSteps(e.Options)
}

func buildInstallSteps(o InstallOptions) []StepInfo {
	var steps []StepInfo
	add := func(desc string) {
		steps = append(steps, StepInfo{Index: len(steps) + 1, Description: desc})
	}
	add("Проверка параметров")
	add("Подготовка папки установки")
	if o.ReinstallExisting {
		add("Копирование файлов")
	}
	if needsDBInitStep(o) {
		add("Инициализация базы данных")
	}
	add("Завершение")
	return steps
}

func needsDBInitStep(o InstallOptions) bool {
	if !(o.Components.InstallDB || o.Components.InstallServer) {
		return false
	}
	return o.DBMode != DBModeKeep
}

func (e *Engine) Done() <-chan struct{} { return e.DoneCh }

func (e *Engine) Cancel() {
	e.cancelOnce.Do(func() {
		close(e.CancelCh)
		CancelActiveInstallerProcesses()
	})
}

func (e *Engine) checkCancelled() error {
	select {
	case <-e.CancelCh:
		return errors.New("установка отменена пользователем")
	default:
		return nil
	}
}

func (e *Engine) writeEmbeddedUninstallerWindows() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if len(e.UninstallerData) == 0 {
		return fmt.Errorf("embedded uninstaller is missing")
	}
	installDir := installDirOrDefault(e.InstallDir)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to prepare install dir for uninstaller: %w", err)
	}
	unPath := filepath.Join(installDir, "cyberstab-uninstaller.exe")
	if err := os.WriteFile(unPath, e.UninstallerData, 0755); err != nil {
		return fmt.Errorf("failed to write uninstaller to %s: %w", unPath, err)
	}
	stat, err := os.Stat(unPath)
	if err != nil {
		return fmt.Errorf("failed to verify uninstaller at %s: %w", unPath, err)
	}
	if stat.Size() == 0 {
		return fmt.Errorf("failed to verify uninstaller at %s: file is empty", unPath)
	}
	refreshWindowsIconCache()
	log.Printf("[INSTALL] Uninstaller written: %s (%d bytes)", unPath, len(e.UninstallerData))
	return nil
}

func (e *Engine) SetUpdateHandler(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.update = fn
}

func (e *Engine) emitUpdate() {
	e.mu.Lock()
	fn := e.update
	e.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (e *Engine) Run() {
	go func() {
		defer e.doneOnce.Do(func() { close(e.DoneCh) })
		e.runningError = e.run()
	}()
}

func (e *Engine) run() error {
	if len(e.Steps) == 0 {
		e.ConfigureSteps()
	}
	stepIdx := 0
	step := func(i int, fn func() error) error {
		if err := e.checkCancelled(); err != nil {
			return err
		}
		e.Steps[i].IsActive = true
		e.emitUpdate()
		err := fn()
		if err != nil {
			e.Steps[i].Error = err
			e.Steps[i].ErrorText = err.Error()
			e.Steps[i].IsActive = false
			e.emitUpdate()
			return err
		}
		if err := e.checkCancelled(); err != nil {
			e.Steps[i].Error = err
			e.Steps[i].ErrorText = err.Error()
			e.Steps[i].IsActive = false
			e.emitUpdate()
			return err
		}
		e.Steps[i].IsActive = false
		e.Steps[i].IsDone = true
		e.emitUpdate()
		return nil
	}
	nextStep := func(fn func() error) error {
		if err := step(stepIdx, fn); err != nil {
			return err
		}
		stepIdx++
		return nil
	}

	// 1) Validate
	if err := nextStep(func() error {
		if strings.TrimSpace(e.Options.SourceRoot) == "" {
			return errors.New("source root directory is required")
		}
		if st, err := os.Stat(e.Options.SourceRoot); err != nil || !st.IsDir() {
			return fmt.Errorf("source root does not exist: %s", e.Options.SourceRoot)
		}
		if !e.Options.Components.InstallServer && !e.Options.Components.InstallClients && !e.Options.Components.InstallDB {
			return errors.New("no components selected")
		}
		return nil
	}); err != nil {
		return err
	}

	// 2) Prepare install dir
	if err := nextStep(func() error {
		if e.InstallDir == "" {
			if runtime.GOOS == "windows" {
				e.InstallDir = `C:\Program Files\Cyberstab`
			} else {
				e.InstallDir = `/opt/cyberstab`
			}
		}
		if err := os.MkdirAll(e.InstallDir, 0755); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// 3) Deploy files (best-effort copy) — skipped when user chose "Оставить" (ReinstallExisting=false).
	if e.Options.ReinstallExisting {
		if err := nextStep(func() error {
		if e.DeployProgressEmitter != nil {
			e.DeployProgressEmitter(0, "Подготовка…")
		}

		installDir := installDirOrDefault(e.InstallDir)

		// "Оставить" in UI sets ReinstallExisting=false: keep files on disk, do not copy from USB.
		if !e.Options.ReinstallExisting {
			_ = stopCyberstabProcesses()
			if e.DeployProgressEmitter != nil {
				e.DeployProgressEmitter(10, "Используются существующие файлы…")
			}
			if err := verifyExistingInstallation(installDir, e.Options.Components); err != nil {
				return err
			}
			if runtime.GOOS == "windows" {
				if e.DeployProgressEmitter != nil {
					e.DeployProgressEmitter(50, "Обновление деинсталлятора…")
				}
				if err := e.writeEmbeddedUninstallerWindows(); err != nil {
					return err
				}
			}
			if e.DeployProgressEmitter != nil {
				e.DeployProgressEmitter(100, "Готово")
			}
			return nil
		}

		// Stop any running Cyberstab processes before copying
		_ = stopCyberstabProcesses()

		// Reinstall: remove old Cyberstab folders, then copy from source.
		if e.DeployProgressEmitter != nil {
			e.DeployProgressEmitter(2, "Удаление старых файлов…")
		}
		_ = cleanOldInstallationFolders(installDir)

		// Copy only Cyberstab distro folders from the selected source root.
		// Do NOT attempt to copy the entire drive root (it contains protected/system dirs).
		dst := e.InstallDir
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
		if runtime.GOOS == "windows" {
			if e.DeployProgressEmitter != nil {
				e.DeployProgressEmitter(3, "Копирование деинсталлятора…")
			}
			if err := e.writeEmbeddedUninstallerWindows(); err != nil {
				return err
			}
		}

		var prefixes []string
		if runtime.GOOS == "windows" {
			if e.Options.Components.InstallServer || e.Options.Components.InstallDB {
				prefixes = append(prefixes, "CyberstabServerWindows")
			}
			if e.Options.Components.InstallClients {
				// Copy only one client by OS bitness.
				if is64BitWindows() {
					prefixes = append(prefixes, "CyberstabClientWindows64")
				} else {
					prefixes = append(prefixes, "CyberstabClientWindows32")
				}
			}
		} else {
			// Linux
			if e.Options.Components.InstallServer || e.Options.Components.InstallDB {
				prefixes = append(prefixes, "CyberstabServerLinux")
			}
			if e.Options.Components.InstallClients {
				// Copy only one client by OS bitness (same as Windows logic).
				if is64BitLinux() {
					prefixes = append(prefixes, "CyberstabClientLinux64")
				} else {
					prefixes = append(prefixes, "CyberstabClientLinux32")
				}
			}
		}
		// Optional extras if present.
		prefixes = append(prefixes, "CyberstabDocumentation")

		selected, selErr := selectTopLevelDirs(e.Options.SourceRoot, prefixes)
		if selErr != nil {
			return selErr
		}
		if len(selected) == 0 {
			return fmt.Errorf("не найдены папки дистрибутива в %s", e.Options.SourceRoot)
		}

		type progState struct{ done, total int64 }
		st := &progState{}
		for _, srcDir := range selected {
			st.total += dirSizeBestEffort(srcDir)
		}

		report := func() {
			if e.DeployProgressEmitter == nil || st.total <= 0 {
				return
			}
			pct := int(float64(st.done) * 100 / float64(st.total))
			if pct > 100 {
				pct = 100
			}
			e.DeployProgressEmitter(pct, "Копирование…")
		}
		report()

		for _, srcDir := range selected {
			target := filepath.Join(dst, filepath.Base(srcDir))
			if err := copyDirBestEffort(srcDir, target, func(delta int64) {
				st.done += delta
				report()
			}); err != nil {
				return err
			}
		}

		if e.DeployProgressEmitter != nil {
			e.DeployProgressEmitter(100, "Готово")
		}

		// Set permissions for client folder IMMEDIATELY after copying
		if runtime.GOOS == "linux" && e.Options.Components.InstallClients {
			_ = setClientFolderPermissionsLinux(dst)
		}
		if runtime.GOOS == "windows" && e.Options.Components.InstallClients {
			log.Printf("[INSTALL] Step 3.1: Setting client folder permissions...")
			log.Printf("[INSTALL] InstallClients = true, dst = %s", dst)
			if e.DeployProgressEmitter != nil {
				e.DeployProgressEmitter(101, "Настройка прав доступа...")
			}
			if err := setClientFolderPermissionsWindows(dst); err != nil {
				log.Printf("[INSTALL] ERROR: Failed to set client permissions: %v", err)
				// Don't fail the whole installation - continue anyway
			} else {
				log.Printf("[INSTALL] Client permissions set successfully")
			}
			flushLog()
		} else {
			log.Printf("[INSTALL] Step 3.1: SKIPPED - InstallClients = %v", e.Options.Components.InstallClients)
			flushLog()
		}

		return nil
		}); err != nil {
			return err
		}
	}

	// 4) DB init: run dbupdater with selected mode — skipped when user chose to keep DB.
	if needsDBInitStep(e.Options) {
		if err := nextStep(func() error {
		if !(e.Options.Components.InstallDB || e.Options.Components.InstallServer) {
			return nil
		}
		if e.ProgressEmitter != nil {
			e.ProgressEmitter(0, "Поиск dbupdater…")
		}

		installDir := installDirOrDefault(e.InstallDir)

		// Step 4.1: Copy server.properties to dbupdater directory
		if e.ProgressEmitter != nil {
			e.ProgressEmitter(2, "Подготовка dbupdater…")
		}
		log.Printf("[DB] Copying server.properties to dbupdater directory...")
		_ = ensureDbUpdaterServerProperties(installDir, filepath.Dir(filepath.Join(installDir, "dbupdater", "dbupdater.exe")))

		log.Printf("[DB] Selected DB mode: %s", e.Options.DBMode)
		switch e.Options.DBMode {
		case DBModeKeep:
			log.Printf("[DB] Keeping existing database: cleanup and dbupdater are skipped")
		case DBModeRecreate, DBModeRestore:
			if e.Options.DBMode == DBModeRecreate || e.Options.DBMode == DBModeRestore {
				if e.ProgressEmitter != nil {
					e.ProgressEmitter(3, "Резервная копия текущей БД…")
				}
				if err := backupOkidociDbIfExistsWindows(e.PgUser, e.PgPassword, installDir); err != nil {
					log.Printf("[DB] Backup before DB cleanup failed: %v", err)
					return fmt.Errorf("не удалось сохранить БД перед удалением: %w", err)
				}
			}

			// Step 4.2: FULL cleanup - drop database and ALL Cyberstab roles
			if e.ProgressEmitter != nil {
				e.ProgressEmitter(4, "Очистка базы данных…")
			}
			log.Printf("[DB] =========================================")
			log.Printf("[DB] Starting FULL database cleanup...")
			log.Printf("[DB] =========================================")

			if err := db.DropOkidociDB(e.PgUser, e.PgPassword); err != nil {
				log.Printf("[DB] ERROR: DropOkidociDB failed: %v", err)
				return fmt.Errorf("failed to drop okidoci_db: %w", err)
			}
			log.Printf("[DB] okidoci_db dropped successfully")

			if err := db.DropCyberstabRoles(e.PgUser, e.PgPassword); err != nil {
				log.Printf("[DB] ERROR: DropCyberstabRoles failed: %v", err)
				// Continue - dbupdater will create roles anyway
			} else {
				log.Printf("[DB] All Cyberstab roles dropped successfully")
			}

			// Give PostgreSQL time to clean up
			time.Sleep(1 * time.Second)

			log.Printf("[DB] =========================================")
			log.Printf("[DB] Database cleanup completed successfully")
			log.Printf("[DB] =========================================")
		default:
			log.Printf("[DB] DB mode %q: no pre-cleanup, dbupdater will apply updates", e.Options.DBMode)
		}

		// Step 4.3: Run dbupdater - it will create database and roles from scratch
		return runDbUpdaterBestEffort(dbUpdaterRunOptions{
			InstallDir: installDir,
			DBMode:     e.Options.DBMode,
			RestoreSQL: strings.TrimSpace(e.Options.DBRestoreFile),
			PGUser:     strings.TrimSpace(e.PgUser),
			PGPassword: strings.TrimSpace(e.PgPassword),
			Cancel:     e.CancelCh,
		}, func(pct int, status string) {
			if e.ProgressEmitter != nil {
				e.ProgressEmitter(pct, status)
			}
		})
		}); err != nil {
			return err
		}
	}

	// 5) Finish
	if err := nextStep(func() error {
		// 5.1) Verify/update embedded uninstaller in install dir.
		if err := e.writeEmbeddedUninstallerWindows(); err != nil {
			return err
		}

		// 5.2) Register apps in Windows "Programs and Features".
		if runtime.GOOS == "windows" {
			// Register only selected components
			hasServer := e.Options.Components.InstallServer || e.Options.Components.InstallDB
			hasClient := e.Options.Components.InstallClients

			_ = registerCyberstabAppsWindows(e.InstallDir, hasServer, hasClient)

			// Clean up old registry entries if component was deselected
			if !hasServer {
				_ = removeUninstallEntryWindows("CyberstabServer")
			}
			if !hasClient {
				_ = removeUninstallEntryWindows("CyberstabClient")
			}
		}

		// 5.3) Create desktop shortcut for client (if installed).
		// Permissions were already set in step 3 after copying files.
		if runtime.GOOS == "windows" && e.Options.Components.InstallClients {
			_ = createClientDesktopShortcutWindows(e.InstallDir)
		}

		// 5.4) Create autostart task and ALWAYS start server (if installed).
		if runtime.GOOS == "windows" && (e.Options.Components.InstallServer || e.Options.Components.InstallDB) {
			if e.ProgressEmitter != nil {
				e.ProgressEmitter(90, "Настройка автозапуска сервера…")
			}

			// Server executable is in server subfolder
			serverExe := filepath.Join(e.InstallDir, "CyberstabServerWindows", "server", "CyberstabServerWindows.exe")
			serverConsole := filepath.Join(e.InstallDir, "CyberstabServerWindows", "serverconsole", "serverconsole.exe")

			// Try server.exe first, fallback to serverconsole.exe
			serverPath := ""
			if _, statErr := os.Stat(serverExe); statErr == nil {
				serverPath = serverExe
			} else if _, statErr := os.Stat(serverConsole); statErr == nil {
				serverPath = serverConsole
			}

			if serverPath != "" {
				// Create autostart task using the server executable
				if err := ensureServerAutostartWindows(e.InstallDir, serverPath); err != nil {
					if e.ProgressEmitter != nil {
						e.ProgressEmitter(92, "Автозапуск: "+err.Error())
					}
				} else {
					if e.ProgressEmitter != nil {
						e.ProgressEmitter(93, "Автозапуск создан")
					}
				}

				// ALWAYS start the server directly (not dependent on any flag)
				if e.ProgressEmitter != nil {
					e.ProgressEmitter(95, "Запуск сервера…")
				}

				// Start server with proper working directory
				cmd := exec.Command(serverPath)
				cmd.Dir = filepath.Dir(serverPath)
				hideCmdDetached(cmd)

				if err := cmd.Start(); err != nil {
					if e.ProgressEmitter != nil {
						e.ProgressEmitter(96, "Ошибка запуска: "+err.Error())
					}
					return fmt.Errorf("не удалось запустить сервер: %w", err)
				} else {
					// Wait until the scheduled task reports running (best-effort) and serverconsole becomes ready.
					if e.ProgressEmitter != nil {
						e.ProgressEmitter(97, "Ожидание запуска задачи…")
					}
					_ = waitForTaskRunningWindows("CyberstabServer", 45*time.Second)

					if e.ProgressEmitter != nil {
						e.ProgressEmitter(98, "Ожидание инициализации сервера…")
					}
					if err := waitForServerReadyWindows(e.InstallDir, 4*time.Minute); err != nil {
						if e.ProgressEmitter != nil {
							e.ProgressEmitter(99, "Сервер не готов: "+err.Error())
						}
						return err
					}
					if e.ProgressEmitter != nil {
						e.ProgressEmitter(99, "Сервер инициализирован")
					}
				}
			} else {
				if e.ProgressEmitter != nil {
					e.ProgressEmitter(90, "Сервер не найден, только автозапуск")
				}
			}
		}

		if runtime.GOOS == "linux" {
			if err := finishInstallLinux(e); err != nil {
				return err
			}
		}

		if e.ProgressEmitter != nil {
			e.ProgressEmitter(100, "Завершено")
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func NeedSudo() bool {
	return needSudo()
}

func (e *Engine) NeedSudo() bool { return NeedSudo() }

func TryRelaunchAsAdmin(args []string) bool {
	return tryRelaunchAsAdmin(args)
}

func InstallPostgresFromSource(sourceRoot string) error {
	// Find an installer exe/msi under sourceRoot and run it.
	var hit string
	_ = filepath.WalkDir(sourceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		n := strings.ToLower(d.Name())
		if (strings.HasSuffix(n, ".exe") || strings.HasSuffix(n, ".msi")) && strings.Contains(n, "postgres") {
			hit = path
			return filepath.SkipAll
		}
		return nil
	})
	if hit == "" {
		return errors.New("postgres installer not found in source")
	}
	cmd := exec.Command(hit)
	if runtime.GOOS == "windows" {
		hideCmd(cmd)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	registerInstallerProcess(cmd.Process)
	go func() {
		_ = cmd.Wait()
		unregisterInstallerProcess(cmd.Process)
	}()
	return nil
}

func selectTopLevelDirs(root string, prefixes []string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		nameLower := strings.ToLower(name)
		for _, p := range prefixes {
			pLower := strings.ToLower(p)
			if nameLower == pLower || strings.HasPrefix(nameLower, pLower) {
				out = append(out, filepath.Join(root, name))
				break
			}
		}
	}
	return out, nil
}

func dirSizeBestEffort(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			// Ignore access errors.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func copyDirBestEffort(src, dst string, onBytes func(delta int64)) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			// Ignore access errors; if it's a dir, prune subtree.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			// Skip Windows protected/system dirs if encountered (defensive).
			name := strings.ToLower(d.Name())
			if name == "system volume information" || name == "$recycle.bin" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			// If file is locked, retry once after a short delay
			time.Sleep(100 * time.Millisecond)
			b, err = os.ReadFile(path)
			if err != nil {
				// Still locked - skip this file and continue
				return nil
			}
		}
		if err := os.WriteFile(target, b, info.Mode()); err != nil {
			// If write fails (file locked), skip and continue
			return nil
		}
		if onBytes != nil {
			onBytes(int64(len(b)))
		}
		return nil
	})
}

func runQuiet(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	cmd.Env = os.Environ()
	_ = cmd.Run()
	return nil
}

var installerProcessRegistry = struct {
	sync.Mutex
	processes map[int]*os.Process
}{
	processes: make(map[int]*os.Process),
}

func registerInstallerProcess(p *os.Process) {
	if p == nil {
		return
	}
	installerProcessRegistry.Lock()
	installerProcessRegistry.processes[p.Pid] = p
	installerProcessRegistry.Unlock()
}

func unregisterInstallerProcess(p *os.Process) {
	if p == nil {
		return
	}
	installerProcessRegistry.Lock()
	delete(installerProcessRegistry.processes, p.Pid)
	installerProcessRegistry.Unlock()
}

func CancelActiveInstallerProcesses() {
	installerProcessRegistry.Lock()
	var processes []*os.Process
	for _, p := range installerProcessRegistry.processes {
		processes = append(processes, p)
	}
	installerProcessRegistry.Unlock()

	for _, p := range processes {
		killProcessTree(p)
	}

	if runtime.GOOS == "windows" {
		for _, name := range []string{"dbupdater.exe", "CyberstabDbUpdaterWindows.exe", "postgresql.exe"} {
			_ = runHidden("taskkill.exe", "/F", "/IM", name, "/T")
		}
	}
	if runtime.GOOS == "linux" {
		for _, name := range []string{"dbupdater", "CyberstabDbUpdaterLinux", "CyberstabServerLinux"} {
			_ = runLinuxCmd("pkill", "-f", name)
		}
	}
}

func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = runHidden("taskkill.exe", "/F", "/PID", strconv.Itoa(p.Pid), "/T")
		return
	}
	_ = p.Kill()
}

type dbUpdaterRunOptions struct {
	InstallDir string
	DBMode     DBMode
	RestoreSQL string
	PGUser     string
	PGPassword string
	Cancel     <-chan struct{}
}

func runDbUpdaterBestEffort(opts dbUpdaterRunOptions, emit func(pct int, status string)) error {
	// If user requested to keep DB, do not run updater.
	if opts.DBMode == DBModeKeep {
		if emit != nil {
			emit(100, "БД: пропуск (оставить текущую)")
		}
		return nil
	}

	installDir := strings.TrimSpace(opts.InstallDir)
	dbu, err := findDbUpdater(installDir)
	if err != nil {
		return err
	}

	if emit != nil {
		emit(5, "Запуск dbupdater…")
	}

	// Restore mode: run -qr <sql>, then run -qu.
	if opts.DBMode == DBModeRestore {
		if strings.TrimSpace(opts.RestoreSQL) == "" {
			return fmt.Errorf("dbupdater: не указан путь к .sql для восстановления")
		}
		if emit != nil {
			emit(10, "Восстановление БД из .sql…")
		}
		if err := runDbUpdaterOnce(dbu, installDir, []string{"-qr", opts.RestoreSQL}, opts.PGUser, opts.PGPassword, emit, 2*time.Hour, "Восстановление БД…", opts.Cancel); err != nil {
			return err
		}
		// After restore, apply updates.
		if emit != nil {
			emit(60, "Применение обновлений БД…")
		}
		return runDbUpdaterOnce(dbu, installDir, []string{"-qu"}, opts.PGUser, opts.PGPassword, emit, 2*time.Hour, "Обновление БД…", opts.Cancel)
	}

	// Default/new/recreate: run update (-qu).
	if emit != nil {
		emit(10, "Создание/обновление БД…")
	}

	// Run dbupdater - it will create database and roles from scratch
	return runDbUpdaterOnce(dbu, installDir, []string{"-qu"}, opts.PGUser, opts.PGPassword, emit, 2*time.Hour, "Инициализация БД…", opts.Cancel)
}

func runDbUpdaterOnce(exePath string, installDir string, args []string, pgUser, pgPassword string, emit func(pct int, status string), timeout time.Duration, tickStatus string, cancel <-chan struct{}) error {
	cmd := exec.Command(exePath, args...)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Env = os.Environ()
	if u := strings.TrimSpace(pgUser); u != "" {
		cmd.Env = append(cmd.Env, "PGUSER="+u)
	}
	if strings.TrimSpace(pgPassword) != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+pgPassword)
	}
	if runtime.GOOS == "windows" {
		hideCmd(cmd)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// dbupdater requires server.properties in its working directory.
	if err := ensureDbUpdaterServerProperties(installDir, cmd.Dir); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("dbupdater: не удалось запустить: %w", err)
	}
	registerInstallerProcess(cmd.Process)
	defer unregisterInstallerProcess(cmd.Process)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	start := time.Now()

	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("dbupdater: ошибка: %v\n%s", err, strings.TrimSpace(out.String()))
			}
			return nil
		case <-cancel:
			killProcessTree(cmd.Process)
			return fmt.Errorf("dbupdater: отменено пользователем")
		case <-ticker.C:
			if emit != nil {
				el := time.Since(start)
				pct := int(el.Seconds() * 99 / timeout.Seconds())
				if pct < 1 {
					pct = 1
				}
				if pct > 99 {
					pct = 99
				}
				emit(pct, tickStatus)
			}
			if time.Since(start) > timeout {
				_ = cmd.Process.Kill()
				return fmt.Errorf("dbupdater: timeout (%s)", timeout.String())
			}
		}
	}
}

func ensureDbUpdaterServerProperties(installDir string, dbUpdaterDir string) error {
	if strings.TrimSpace(dbUpdaterDir) == "" {
		return fmt.Errorf("dbupdater: invalid work dir")
	}
	target := filepath.Join(dbUpdaterDir, "server.properties")
	if st, err := os.Stat(target); err == nil && !st.IsDir() {
		return nil
	}

	// Try to locate server.properties under installed server folder.
	src, err := findServerProperties(installDir)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("dbupdater: не удалось прочитать %s: %w", src, err)
	}
	if err := os.WriteFile(target, b, 0644); err != nil {
		return fmt.Errorf("dbupdater: не удалось записать %s: %w", target, err)
	}
	return nil
}

func findServerProperties(installDir string) (string, error) {
	// Common locations:
	// - <installDir>\CyberstabServerWindows*\server.properties
	// - <installDir>\CyberstabServerWindows*\config\server.properties
	// - <installDir>\server.properties
	candidates := []string{
		filepath.Join(installDir, "server.properties"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}

	var hit string
	_ = filepath.WalkDir(installDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(installDir, path)
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) > 5 {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), "server.properties") {
			hit = path
			return filepath.SkipAll
		}
		return nil
	})
	if hit == "" {
		return "", fmt.Errorf("dbupdater: файл server.properties не найден в %s", installDir)
	}
	return hit, nil
}

func findDbUpdater(installDir string) (string, error) {
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			filepath.Join(installDir, "dbupdater", "dbupdater.exe"),
			filepath.Join(installDir, "dbupdater", "CyberstabDbUpdaterWindows.exe"),
			filepath.Join(installDir, "dbupdater.exe"),
			filepath.Join(installDir, "CyberstabDbUpdaterWindows.exe"),
		}
	} else {
		candidates = []string{
			filepath.Join(installDir, "dbupdater", "dbupdater"),
			filepath.Join(installDir, "dbupdater", "CyberstabDbUpdaterLinux"),
			filepath.Join(installDir, "dbupdater"),
			filepath.Join(installDir, "CyberstabDbUpdaterLinux"),
			filepath.Join(installDir, "CyberstabServerLinux", "dbupdater", "dbupdater"),
			filepath.Join(installDir, "CyberstabServerLinux", "dbupdater", "CyberstabDbUpdaterLinux"),
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}

	var hit string
	_ = filepath.WalkDir(installDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(installDir, path)
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if runtime.GOOS == "windows" {
			if name == "dbupdater.exe" || name == "cyberstabdbupdaterwindows.exe" || strings.Contains(name, "dbupdater") && strings.HasSuffix(name, ".exe") {
				hit = path
				return filepath.SkipAll
			}
		} else if name == "dbupdater" || name == "cyberstabdbupdaterlinux" || strings.Contains(name, "dbupdater") {
			hit = path
			return filepath.SkipAll
		}
		return nil
	})
	if hit == "" {
		return "", fmt.Errorf("dbupdater не найден в %s", installDir)
	}
	return hit, nil
}

func installDirOrDefault(p string) string {
	if strings.TrimSpace(p) != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		return `C:\Program Files\Cyberstab`
	}
	return `/opt/cyberstab`
}

func is64BitWindows() bool {
	// Installer is built as amd64 for your target, but keep a runtime check.
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		return true
	}
	arch := strings.ToLower(os.Getenv("PROCESSOR_ARCHITECTURE"))
	archw := strings.ToLower(os.Getenv("PROCESSOR_ARCHITEW6432"))
	return strings.Contains(arch, "64") || strings.Contains(archw, "64")
}

func is64BitLinux() bool {
	// Check Linux architecture
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		return true
	}
	// Fallback: try to read from /proc/cpuinfo or uname
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err != nil {
		// Default to 64-bit if we can't determine (safer default)
		return true
	}
	arch := strings.TrimSpace(string(output))
	return strings.Contains(arch, "64") || strings.Contains(arch, "x86_64") || strings.Contains(arch, "aarch64")
}

func verifyExistingInstallation(installDir string, components ComponentsOptions) error {
	var missing []string
	if runtime.GOOS == "windows" {
		if components.InstallServer || components.InstallDB {
			serverDir := filepath.Join(installDir, "CyberstabServerWindows")
			if st, err := os.Stat(serverDir); err != nil || !st.IsDir() {
				missing = append(missing, "CyberstabServerWindows")
			} else if components.InstallDB {
				if _, err := findDbUpdater(installDir); err != nil {
					missing = append(missing, "dbupdater (в CyberstabServerWindows)")
				}
			}
		}
		if components.InstallClients {
			clientDir := ""
			if is64BitWindows() {
				clientDir = filepath.Join(installDir, "CyberstabClientWindows64")
			} else {
				clientDir = filepath.Join(installDir, "CyberstabClientWindows32")
			}
			if st, err := os.Stat(clientDir); err != nil || !st.IsDir() {
				missing = append(missing, filepath.Base(clientDir))
			}
		}
	} else {
		if components.InstallServer || components.InstallDB {
			serverDir := filepath.Join(installDir, "CyberstabServerLinux")
			if st, err := os.Stat(serverDir); err != nil || !st.IsDir() {
				missing = append(missing, "CyberstabServerLinux")
			}
		}
		if components.InstallClients {
			clientDir := ""
			if is64BitLinux() {
				clientDir = filepath.Join(installDir, "CyberstabClientLinux64")
			} else {
				clientDir = filepath.Join(installDir, "CyberstabClientLinux32")
			}
			if st, err := os.Stat(clientDir); err != nil || !st.IsDir() {
				missing = append(missing, filepath.Base(clientDir))
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("в папке установки не найдены компоненты: %s. Выберите «Переустановить» или скопируйте дистрибутив вручную", strings.Join(missing, ", "))
	}
	return nil
}

// cleanOldInstallationFolders removes old Cyberstab folders before reinstall
func cleanOldInstallationFolders(installDir string) error {
	// List of folders to clean
	folders := []string{
		"CyberstabServerWindows",
		"CyberstabServerLinux",
		"CyberstabClientWindows32",
		"CyberstabClientWindows64",
		"CyberstabClientLinux32",
		"CyberstabClientLinux64",
		"dbupdater",
	}

	for _, folder := range folders {
		path := filepath.Join(installDir, folder)
		if _, err := os.Stat(path); err == nil {
			// Try to remove the folder with retries (in case files are locked)
			for i := 0; i < 3; i++ {
				err := os.RemoveAll(path)
				if err == nil {
					break
				}
				// If failed, wait and retry
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	return nil
}

// stopCyberstabProcesses terminates any running Cyberstab processes to avoid file locks during installation.
func stopCyberstabProcesses() error {
	if runtime.GOOS == "linux" {
		return stopCyberstabProcessesLinux()
	}
	if runtime.GOOS != "windows" {
		return nil
	}

	// First, try to stop the scheduled task if it's running
	_ = runHidden("schtasks.exe", "/End", "/TN", "CyberstabServer")
	time.Sleep(300 * time.Millisecond)

	// List of process names to terminate (order matters - stop server first)
	processNames := []string{
		"CyberstabServerWindows.exe", // Main server
		"serverconsole.exe",          // Server console
		"CyberstabClientWindows.exe", // Client
	}

	for _, procName := range processNames {
		// Use taskkill to terminate processes
		cmd := exec.Command("taskkill.exe", "/F", "/IM", procName, "/T")
		hideCmd(cmd)
		_ = cmd.Run()                      // Best effort, ignore errors
		time.Sleep(200 * time.Millisecond) // Give each process time to exit
	}

	// Also try to stop Java processes that are running from Cyberstab directory
	// This is more aggressive - only if other processes didn't stop
	cmd := exec.Command("taskkill.exe", "/F", "/IM", "java.exe", "/T")
	hideCmd(cmd)
	_ = cmd.Run()

	// Give processes a moment to release file handles
	time.Sleep(500 * time.Millisecond)

	return nil
}

// createEmptyOkidociDB creates an empty okidoci_db database with basic roles
func createEmptyOkidociDB(pgPassword string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	info, err := db.CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return fmt.Errorf("PostgreSQL not found: %v", err)
	}

	psql := filepath.Join(info.BinDir, "psql")
	if runtime.GOOS == "windows" {
		psql += ".exe"
	}

	// Helper to run psql
	runPsql := func(dbName, sql string) error {
		cmd := exec.Command(psql, "-U", "postgres", "-d", dbName, "-c", sql)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+pgPassword)
		hideCmd(cmd)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		log.Printf("[DB] Running on %s: %s", dbName, sql)

		if err := cmd.Run(); err != nil {
			log.Printf("[DB] ERROR: %v", err)
			if stderr.Len() > 0 {
				log.Printf("[DB] stderr: %s", strings.TrimSpace(stderr.String()))
			}
			return err
		}

		if stdout.Len() > 0 {
			log.Printf("[DB] stdout: %s", strings.TrimSpace(stdout.String()))
		}
		return nil
	}

	// Step 1: Create database
	log.Printf("[DB] Creating okidoci_db...")
	if err := runPsql("postgres", "CREATE DATABASE okidoci_db;"); err != nil {
		log.Printf("[DB] Warning: CREATE DATABASE failed (may already exist): %v", err)
	}

	// Step 2: Create basic roles with passwords from server.properties if available
	// Otherwise use defaults - dbupdater will update them
	log.Printf("[DB] Creating basic roles...")

	// Try to read passwords from server.properties
	installDir := installDirOrDefault("")
	serverPropsPath := filepath.Join(installDir, "server.properties")
	props := readServerProperties(serverPropsPath)

	adminPass := props["okidoci.admin.password"]
	servicePass := props["okidoci.service_user.password"]
	usersPass := props["okidoci.users.password"]

	// Fallback to defaults if not found
	if adminPass == "" {
		adminPass = "admin"
	}
	if servicePass == "" {
		servicePass = "service"
	}
	if usersPass == "" {
		usersPass = "users"
	}

	log.Printf("[DB] Using admin password: %s", maskPassword(adminPass))

	roles := []struct {
		name string
		pass string
	}{
		{"okidoci_admin", adminPass},
		{"okidoci_service_user_name", servicePass},
		{"okidoci_users", usersPass},
	}

	for _, role := range roles {
		// Drop if exists, then create
		_ = runPsql("postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", role.name))

		createSQL := fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD '%s';", role.name, role.pass)
		if err := runPsql("postgres", createSQL); err != nil {
			log.Printf("[DB] Warning: Failed to create role %s: %v", role.name, err)
		}
	}

	// Grant connect privilege
	if err := runPsql("postgres", "GRANT ALL PRIVILEGES ON DATABASE okidoci_db TO okidoci_admin;"); err != nil {
		log.Printf("[DB] Warning: GRANT failed: %v", err)
	}

	log.Printf("[DB] Empty database created successfully")
	return nil
}

// maskPassword hides password for logging
func maskPassword(p string) string {
	if len(p) <= 2 {
		return "***"
	}
	return p[:1] + "***" + p[len(p)-1:]
}

// readServerProperties reads key=value from properties file
func readServerProperties(path string) map[string]string {
	props := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return props
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			props[key] = val
		}
	}
	return props
}

// setClientFolderPermissionsWindows grants Full Control permissions to all users for the client folder.
// This ensures any user can launch the client, not just administrators.
func setClientFolderPermissionsWindows(installDir string) error {
	// Force flush logs before starting
	flushLog()
	log.Printf("[PERMS] >>>>> STARTING permission setup for installDir: %s", installDir)
	flushLog()

	// Collect all client directories
	var clientDirs []string

	// Check 64-bit client
	dir64 := filepath.Join(installDir, "CyberstabClientWindows64")
	if _, err := os.Stat(dir64); err == nil {
		clientDirs = append(clientDirs, dir64)
		log.Printf("[PERMS] Found 64-bit client dir: %s", dir64)
	}

	// Check 32-bit client
	dir32 := filepath.Join(installDir, "CyberstabClientWindows32")
	if _, err := os.Stat(dir32); err == nil {
		clientDirs = append(clientDirs, dir32)
		log.Printf("[PERMS] Found 32-bit client dir: %s", dir32)
	}

	// Fallback to DetectClientDirWindows
	if len(clientDirs) == 0 {
		if dir := DetectClientDirWindows(installDir); dir != "" {
			clientDirs = append(clientDirs, dir)
			log.Printf("[PERMS] Using detected client dir: %s", dir)
		}
	}

	if len(clientDirs) == 0 {
		log.Printf("[PERMS] ERROR: No client directories found in %s", installDir)
		flushLog()
		return nil
	}

	log.Printf("[PERMS] Processing %d client director(ies)", len(clientDirs))
	flushLog()

	// Set permissions for each client directory
	for i, clientDir := range clientDirs {
		log.Printf("[PERMS] [%d/%d] =========================================", i+1, len(clientDirs))
		log.Printf("[PERMS] [%d/%d] Processing: %s", i+1, len(clientDirs), clientDir)
		flushLog()

		// Method 1: Use PowerShell with Get-Acl/Set-Acl
		log.Printf("[PERMS] [%d/%d] Method 1: Using PowerShell...", i+1, len(clientDirs))

		psScript := fmt.Sprintf(`
			$ErrorActionPreference = "Continue"
			$path = "%s"
			
			# Get current ACL
			$acl = Get-Acl -Path $path
			
			# Create Full Control rule for Everyone
			$everyone = New-Object System.Security.Principal.SecurityIdentifier("S-1-1-0")
			$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
				$everyone,
				"FullControl",
				"ContainerInherit,ObjectInherit",
				"None",
				"Allow"
			)
			
			# Add the rule
			$acl.SetAccessRule($rule)
			
			# Apply recursively
			Set-Acl -Path $path -AclObject $acl -ErrorAction Continue
			
			# Also apply to all child items
			Get-ChildItem -Path $path -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object {
				try {
					$itemAcl = Get-Acl -Path $_.FullName
					$itemAcl.SetAccessRule($rule)
					Set-Acl -Path $_.FullName -AclObject $itemAcl -ErrorAction Continue
				} catch {
					Write-Host "Warning: Could not set ACL on $($_.FullName): $_"
				}
			}
			
			Write-Host "Permissions set successfully"
		`, clientDir)

		cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript)
		hideCmd(cmd)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			log.Printf("[PERMS] [%d/%d] PowerShell failed: %v", i+1, len(clientDirs), err)
			if out.Len() > 0 {
				log.Printf("[PERMS] [%d/%d] PowerShell output: %s", i+1, len(clientDirs), strings.TrimSpace(out.String()))
			}

			// Method 2: Fallback to icacls.exe
			log.Printf("[PERMS] [%d/%d] Method 2: Using icacls.exe fallback...", i+1, len(clientDirs))
			cmd = exec.Command("icacls.exe", clientDir, "/grant:r", "Everyone:(OI)(CI)(F)", "/T", "/C")
			hideCmd(cmd)
			out.Reset()
			cmd.Stdout = &out
			cmd.Stderr = &out

			if err := cmd.Run(); err != nil {
				log.Printf("[PERMS] [%d/%d] ERROR: Both PowerShell and icacls failed", i+1, len(clientDirs))
				if out.Len() > 0 {
					log.Printf("[PERMS] [%d/%d] icacls stderr: %s", i+1, len(clientDirs), strings.TrimSpace(out.String()))
				}
			} else {
				log.Printf("[PERMS] [%d/%d] Success via icacls.exe fallback", i+1, len(clientDirs))
			}
		} else {
			log.Printf("[PERMS] [%d/%d] Success via PowerShell", i+1, len(clientDirs))
			if out.Len() > 0 {
				log.Printf("[PERMS] [%d/%d] PowerShell output: %s", i+1, len(clientDirs), strings.TrimSpace(out.String()))
			}
		}

		flushLog()

		// Verify permissions
		log.Printf("[PERMS] [%d/%d] Verifying permissions...", i+1, len(clientDirs))
		cmd = exec.Command("icacls.exe", clientDir)
		hideCmd(cmd)
		out.Reset()
		cmd.Stdout = &out
		cmd.Stderr = &out
		_ = cmd.Run()
		if out.Len() > 0 {
			log.Printf("[PERMS] [%d/%d] Current ACL:\n%s", i+1, len(clientDirs), out.String())
		}
		flushLog()

		log.Printf("[PERMS] [%d/%d] Finished: %s", i+1, len(clientDirs), clientDir)
		log.Printf("[PERMS] [%d/%d] =========================================", i+1, len(clientDirs))
		flushLog()
	}

	log.Printf("[PERMS] <<<<< All permissions set successfully")
	flushLog()
	return nil
}

// flushLog forces pending log output to be written to disk
func flushLog() {
	// Get the underlying file if it's wrapped
	if logger := log.Writer(); logger != nil {
		if f, ok := logger.(*os.File); ok {
			_ = f.Sync()
		}
	}
}

// runCmdWithOutputArgs runs a command and logs stdout/stderr
func runCmdWithOutputArgs(cmd *exec.Cmd, prefix string) error {
	hideCmd(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("%s Running: %s %v", prefix, cmd.Path, cmd.Args)

	err := cmd.Run()

	if stdout.Len() > 0 {
		log.Printf("%s stdout: %s", prefix, strings.TrimSpace(stdout.String()))
	}
	if stderr.Len() > 0 {
		log.Printf("%s stderr: %s", prefix, strings.TrimSpace(stderr.String()))
	}

	return err
}

// --- Windows integration helpers (best-effort) ---

func registerCyberstabAppsWindows(installDir string, hasServer bool, hasClient bool) error {
	// Uses reg.exe to avoid direct registry API here (keeps deps small).
	// Creates two entries: server and client, if corresponding folders exist.
	if hasServer {
		ver := detectJavaVersion(filepath.Join(installDir, "CyberstabServerWindows", "java"))
		uninstallerPath := filepath.Join(installDir, "cyberstab-uninstaller.exe")
		_ = writeUninstallEntryWindows("CyberstabServer", "Киберстаб (сервер)", ver, installDir, `"`+uninstallerPath+`" -y`, uninstallerPath)
	}
	if hasClient {
		clientDir := DetectClientDirWindows(installDir)
		ver := detectJavaVersion(filepath.Join(clientDir, "java"))
		uninstallerPath := filepath.Join(installDir, "cyberstab-uninstaller.exe")
		_ = writeUninstallEntryWindows("CyberstabClient", "Киберстаб (клиент)", ver, installDir, `"`+uninstallerPath+`" -y`, uninstallerPath)
	}
	return nil
}

func removeUninstallEntryWindows(keyName string) error {
	base := `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\` + keyName
	// Delete the entire registry key
	return runHidden("reg.exe", "DELETE", base, "/f")
}

func writeUninstallEntryWindows(keyName string, displayName string, version string, installDir string, uninstallCmd string, iconPath string) error {
	base := `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\` + keyName
	// Minimal fields from your screenshot.
	_ = runHidden("reg.exe", "ADD", base, "/f")
	_ = runHidden("reg.exe", "ADD", base, "/v", "DisplayName", "/t", "REG_SZ", "/d", displayName, "/f")
	if version != "" {
		_ = runHidden("reg.exe", "ADD", base, "/v", "DisplayVersion", "/t", "REG_SZ", "/d", version, "/f")
	}
	_ = runHidden("reg.exe", "ADD", base, "/v", "Publisher", "/t", "REG_SZ", "/d", `ООО "Стандарт безопасности"`, "/f")
	_ = runHidden("reg.exe", "ADD", base, "/v", "InstallLocation", "/t", "REG_SZ", "/d", installDir, "/f")
	_ = runHidden("reg.exe", "ADD", base, "/v", "HelpLink", "/t", "REG_SZ", "/d", "https://kiberstab.ru/", "/f")
	_ = runHidden("reg.exe", "ADD", base, "/v", "URLInfoAbout", "/t", "REG_SZ", "/d", "https://kiberstab.ru/", "/f")
	if strings.TrimSpace(iconPath) != "" {
		_ = runHidden("reg.exe", "ADD", base, "/v", "DisplayIcon", "/t", "REG_SZ", "/d", `"`+iconPath+`",0`, "/f")
	}
	_ = runHidden("reg.exe", "ADD", base, "/v", "UninstallString", "/t", "REG_SZ", "/d", uninstallCmd, "/f")
	_ = runHidden("reg.exe", "ADD", base, "/v", "QuietUninstallString", "/t", "REG_SZ", "/d", uninstallCmd, "/f")
	return nil
}

func detectJavaVersion(javaDir string) string {
	entries, err := os.ReadDir(javaDir)
	if err != nil {
		return ""
	}
	var vers []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v := strings.TrimSpace(e.Name())
		if looksLikeSemver(v) {
			vers = append(vers, v)
		}
	}
	if len(vers) == 0 {
		return ""
	}
	sort.Slice(vers, func(i, j int) bool { return semverLess(vers[i], vers[j]) })
	return vers[len(vers)-1]
}

func looksLikeSemver(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func semverLess(a, b string) bool {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	n := int(math.Max(float64(len(ap)), float64(len(bp))))
	for len(ap) < n {
		ap = append(ap, "0")
	}
	for len(bp) < n {
		bp = append(bp, "0")
	}
	for i := 0; i < n; i++ {
		ai, _ := strconv.Atoi(ap[i])
		bi, _ := strconv.Atoi(bp[i])
		if ai != bi {
			return ai < bi
		}
	}
	return false
}

func DetectClientDirWindows(installDir string) string {
	if is64BitWindows() {
		return filepath.Join(installDir, "CyberstabClientWindows64")
	}
	return filepath.Join(installDir, "CyberstabClientWindows32")
}

func createClientDesktopShortcutWindows(installDir string) error {
	clientDir := DetectClientDirWindows(installDir)
	if clientDir == "" {
		return fmt.Errorf("client directory not found")
	}

	targetExe := FindClientExeBestEffort(clientDir)
	if targetExe == "" {
		return fmt.Errorf("client executable not found in %s", clientDir)
	}

	log.Printf("[SHORTCUT] Creating shortcut for client: %s", targetExe)

	// Get public desktop (common for all users)
	desktop := os.Getenv("PUBLIC")
	if desktop == "" {
		desktop = filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	} else {
		desktop = filepath.Join(desktop, "Desktop")
	}

	// Ensure desktop directory exists
	if err := os.MkdirAll(desktop, 0755); err != nil {
		return fmt.Errorf("failed to create desktop directory: %w", err)
	}

	lnk := filepath.Join(desktop, "Киберстаб.lnk")

	log.Printf("[SHORTCUT] Desktop path: %s", lnk)

	// Create shortcut with icon
	ps := fmt.Sprintf(`
		$W = New-Object -ComObject WScript.Shell;
		$S = $W.CreateShortcut('%s');
		$S.TargetPath = '%s';
		$S.WorkingDirectory = '%s';
		$S.IconLocation = '%s, 0';
		$S.Description = 'Киберстаб Клиент';
		$S.Save();
	`, escapePS(lnk), escapePS(targetExe), escapePS(filepath.Dir(targetExe)), escapePS(targetExe))

	if err := runHidden("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps); err != nil {
		return fmt.Errorf("failed to create shortcut: %w", err)
	}

	log.Printf("[SHORTCUT] Shortcut created, setting permissions...")

	// Method 1: Use icacls through cmd.exe to grant Read permissions to Users
	cmdLine := fmt.Sprintf(`icacls.exe "%s" /grant:r *S-1-5-32-545:(R) /Q`, lnk)
	cmd := exec.Command("cmd.exe", "/C", cmdLine)
	hideCmd(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		log.Printf("[SHORTCUT] icacls Users failed: %v", err)
		if out.Len() > 0 {
			log.Printf("[SHORTCUT] icacls output: %s", strings.TrimSpace(out.String()))
		}

		// Method 2: Try Everyone
		cmdLine = fmt.Sprintf(`icacls.exe "%s" /grant:r Everyone:(R) /Q`, lnk)
		cmd = exec.Command("cmd.exe", "/C", cmdLine)
		hideCmd(cmd)
		out.Reset()
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			log.Printf("[SHORTCUT] icacls Everyone failed: %v", err)
			if out.Len() > 0 {
				log.Printf("[SHORTCUT] icacls output: %s", strings.TrimSpace(out.String()))
			}
		} else {
			log.Printf("[SHORTCUT] Permissions set via Everyone")
		}
	} else {
		log.Printf("[SHORTCUT] Permissions set via Users group")
	}

	// Verify the file exists and check its ACL
	if info, err := os.Stat(lnk); err == nil {
		log.Printf("[SHORTCUT] Shortcut file exists, size: %d bytes", info.Size())
	}

	log.Printf("[SHORTCUT] Shortcut creation completed")
	return nil
}

func FindClientExeBestEffort(clientDir string) string {
	// Search for client executable at shallow depth.
	// Priority order:
	// 1. Files with "client" in name
	// 2. Any .exe in the root of clientDir
	// 3. Any .exe at depth 1

	var priorityHits []string
	var rootExe []string
	var depth1Exe []string

	_ = filepath.WalkDir(clientDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(clientDir, path)
		depth := strings.Count(rel, string(os.PathSeparator))

		// Don't go deeper than 2 levels
		if depth > 2 {
			return filepath.SkipDir
		}

		if d.IsDir() {
			// Skip java and other large directories
			name := strings.ToLower(d.Name())
			if name == "java" || name == "jre" || name == "jdk" {
				return filepath.SkipDir
			}
			return nil
		}

		name := strings.ToLower(d.Name())
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(name, ".exe") {
				return nil
			}
		} else if d.IsDir() || !isLikelyExecutable(path, d) {
			return nil
		}

		// Collect by priority
		if strings.Contains(name, "client") {
			priorityHits = append(priorityHits, path)
		} else if depth == 0 {
			rootExe = append(rootExe, path)
		} else if depth == 1 {
			depth1Exe = append(depth1Exe, path)
		}

		return nil
	})

	// Return by priority
	if len(priorityHits) > 0 {
		sort.Strings(priorityHits)
		return priorityHits[0]
	}
	if len(rootExe) > 0 {
		sort.Strings(rootExe)
		return rootExe[0]
	}
	if len(depth1Exe) > 0 {
		sort.Strings(depth1Exe)
		return depth1Exe[0]
	}

	return ""
}

func ensureServerAutostartWindows(installDir string, serverPath string) error {
	// Create a scheduled task to run server at startup with admin rights.
	if _, err := os.Stat(serverPath); err != nil {
		return fmt.Errorf("сервер не найден: %w", err)
	}

	taskName := "CyberstabServer"
	// Quote the path to handle spaces correctly.
	tr := fmt.Sprintf(`"%s"`, serverPath)

	// Delete old task if exists (ensures clean state)
	_ = runHidden("schtasks.exe", "/Delete", "/TN", taskName, "/F")
	time.Sleep(300 * time.Millisecond)

	// Create new task to run at startup with highest privileges.
	// Use SYSTEM account to avoid user/session issues.
	err := runHidden(
		"schtasks.exe",
		"/Create",
		"/F",
		"/TN", taskName,
		"/SC", "ONSTART",
		"/RU", "SYSTEM",
		"/RL", "HIGHEST",
		"/TR", tr,
	)
	if err != nil {
		return fmt.Errorf("не удалось создать задачу автозапуска: %w", err)
	}

	// Verify task was created
	var out bytes.Buffer
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName, "/V", "/FO", "LIST")
	hideCmd(cmd)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err == nil {
		// Task exists
		return nil
	}

	return fmt.Errorf("задача создана, но не найдена при проверке")
}

func waitForTaskRunningWindows(taskName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := runCmdWithTimeout(6*time.Second, "schtasks.exe", "/Query", "/TN", taskName, "/FO", "LIST", "/V")
		if err == nil {
			raw := strings.ToLower(out)
			if strings.Contains(raw, "status:") && strings.Contains(raw, "running") {
				return nil
			}
		}
		time.Sleep(1200 * time.Millisecond)
	}
	return fmt.Errorf("таймаут ожидания запуска задачи %s", taskName)
}

func waitForServerReadyWindows(installDir string, timeout time.Duration) error {
	// Readiness check by log marker.
	// Server log path:
	//   <installDir>\CyberstabServerWindows\log\server\error_log.log
	// Wait until:
	//   [main] NetworkMessageDispatcher started
	logPath := filepath.Join(installDir, "CyberstabServerWindows", "log", "server", "error_log.log")
	return waitForLogLineWindows(logPath, "[main] NetworkMessageDispatcher started", timeout)
}

func waitForLogLineWindows(logPath string, needle string, timeout time.Duration) error {
	if strings.TrimSpace(logPath) == "" {
		return fmt.Errorf("log path is empty")
	}
	if strings.TrimSpace(needle) == "" {
		return fmt.Errorf("needle is empty")
	}

	deadline := time.Now().Add(timeout)

	// We re-read the tail of the file. This is robust and doesn't require file sharing flags.
	var lastSize int64 = -1
	var stableCount int

	for time.Now().Before(deadline) {
		fi, err := os.Stat(logPath)
		if err != nil {
			// Log file may appear later.
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		size := fi.Size()
		if lastSize == size {
			stableCount++
		} else {
			stableCount = 0
		}
		lastSize = size

		// Read last up to 512KB (enough for recent startup logs, avoids huge reads).
		const maxTail = int64(512 * 1024)
		start := int64(0)
		if size > maxTail {
			start = size - maxTail
		}

		f, err := os.Open(logPath)
		if err != nil {
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		_, _ = f.Seek(start, 0)
		b, _ := io.ReadAll(f)
		_ = f.Close()

		if bytes.Contains(b, []byte(needle)) {
			return nil
		}

		// If log isn't growing for a while, back off slightly.
		if stableCount > 5 {
			time.Sleep(1800 * time.Millisecond)
		} else {
			time.Sleep(900 * time.Millisecond)
		}
	}
	return fmt.Errorf("таймаут ожидания строки в логе: %s", needle)
}

func startServerTaskWindows() error {
	// Try multiple times with delays
	for i := 0; i < 3; i++ {
		err := runHidden("schtasks.exe", "/Run", "/TN", "CyberstabServer")
		if err == nil {
			// Wait for the task to actually start
			time.Sleep(2 * time.Second)
			return nil
		}

		// If first attempt fails, wait a bit and retry
		if i < 2 {
			time.Sleep(1 * time.Second)
		}
	}

	return fmt.Errorf("не удалось запустить задачу CyberstabServer после 3 попыток")
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func runHidden(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	hideCmd(cmd)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(b.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func runCmdWithTimeout(timeout time.Duration, exe string, args ...string) (string, error) {
	cmd := exec.Command(exe, args...)
	hideCmd(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		out := stdout.String()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return out, fmt.Errorf("%s", msg)
		}
		return out, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return stdout.String(), fmt.Errorf("timeout")
	}
}

func backupOkidociDbIfExistsWindows(pgUser, pgPassword string, installDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if strings.TrimSpace(pgPassword) == "" {
		return fmt.Errorf("backup: нужен пароль PostgreSQL для резервного копирования")
	}
	exists, err := db.OkidociDatabaseExists(pgUser, pgPassword)
	if err != nil || !exists {
		return nil
	}
	ts := time.Now().Format("20060102_150405")
	outDir := filepath.Join(installDir, "backups", "db")
	outFile := filepath.Join(outDir, "okidoci_db_"+ts+".sql")
	return db.DumpDatabaseSQL(pgUser, pgPassword, "okidoci_db", outFile)
}
