package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

type PostgresInfo struct {
	Installed bool
	BinDir    string
}

type EngineKind string

const (
	EnginePostgreSQL EngineKind = "postgresql"
	EngineJatoba     EngineKind = "jatoba"
)

type EngineInfo struct {
	Kind        EngineKind
	DisplayName string
	BinDir      string
	RootDir     string
}

var postgresBinDir string
var activeEngine EngineInfo

func SetPostgresBinDir(dir string) {
	postgresBinDir = strings.TrimSpace(dir)
	if postgresBinDir == "" {
		activeEngine = EngineInfo{}
		return
	}
	kind := detectEngineKindByPath(postgresBinDir)
	activeEngine = EngineInfo{
		Kind:        kind,
		DisplayName: engineDisplayName(kind),
		BinDir:      postgresBinDir,
		RootDir:     filepath.Dir(postgresBinDir),
	}
}

func engineDisplayName(kind EngineKind) string {
	if kind == EngineJatoba {
		return "Jatoba"
	}
	return "PostgreSQL"
}

func detectEngineKindByPath(binDir string) EngineKind {
	p := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(binDir), "/", `\`))
	if strings.Contains(p, `\gis\jatoba\`) || strings.Contains(p, `\jatoba\`) {
		return EngineJatoba
	}
	return EnginePostgreSQL
}

func DiscoverEngines() ([]EngineInfo, error) {
	var out []EngineInfo
	seen := map[string]bool{}
	add := func(kind EngineKind, binDir string) {
		binDir = strings.TrimSpace(binDir)
		if binDir == "" {
			return
		}
		key := strings.ToLower(filepath.Clean(binDir))
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, EngineInfo{
			Kind:        kind,
			DisplayName: engineDisplayName(kind),
			BinDir:      binDir,
			RootDir:     filepath.Dir(binDir),
		})
	}
	if bin, err := discoverPostgresBin(); err == nil {
		add(EnginePostgreSQL, bin)
	}
	if bin, err := discoverJatobaBin(); err == nil {
		add(EngineJatoba, bin)
	}
	if strings.TrimSpace(postgresBinDir) != "" {
		add(detectEngineKindByPath(postgresBinDir), postgresBinDir)
	}
	return out, nil
}

func SelectEngineByKind(kind EngineKind) (EngineInfo, error) {
	engines, err := DiscoverEngines()
	if err != nil {
		return EngineInfo{}, err
	}
	for _, e := range engines {
		if e.Kind == kind {
			SetPostgresBinDir(e.BinDir)
			return e, nil
		}
	}
	return EngineInfo{}, fmt.Errorf("движок %s не найден", kind)
}

func GetActiveEngine() EngineInfo {
	if strings.TrimSpace(activeEngine.BinDir) != "" {
		return activeEngine
	}
	engines, _ := DiscoverEngines()
	if len(engines) > 0 {
		return engines[0]
	}
	return EngineInfo{}
}

func CheckPostgres() (*PostgresInfo, error) {
	bin := postgresBinDir
	if strings.TrimSpace(bin) == "" {
		engines, err := DiscoverEngines()
		if err != nil || len(engines) == 0 {
			return &PostgresInfo{Installed: false}, nil
		}
		bin = engines[0].BinDir
		postgresBinDir = bin
		activeEngine = engines[0]
	}
	psql := filepath.Join(bin, "psql")
	if runtime.GOOS == "windows" {
		psql += ".exe"
	}
	if _, err := os.Stat(psql); err != nil {
		return &PostgresInfo{Installed: false}, nil
	}
	return &PostgresInfo{Installed: true, BinDir: bin}, nil
}

func normalizePgUser(user string) string {
	u := strings.TrimSpace(user)
	if u == "" {
		return "postgres"
	}
	return u
}

// VerifyPostgresCredentials checks login and whether the user can create SUPERUSER roles (required by dbupdater).
func VerifyPostgresCredentials(user, password string) error {
	user = normalizePgUser(user)
	if _, err := runPSQLAuth(user, password, "postgres", "SELECT 1;"); err != nil {
		return friendlyCredentialError(user, err.Error())
	}
	if err := verifyCanCreateSuperuserRole(user, password); err != nil {
		return err
	}
	return nil
}

func verifyCanCreateSuperuserRole(user, password string) error {
	out, err := runPSQLAuth(user, password, "postgres", "SELECT COALESCE(rolsuper, false) FROM pg_roles WHERE rolname = current_user;")
	if err != nil {
		return friendlyCredentialError(user, err.Error())
	}
	if strings.TrimSpace(out) == "t" {
		return nil
	}
	testRole := fmt.Sprintf("_cyberstab_chk_%d", time.Now().UnixNano())
	createSQL := fmt.Sprintf("CREATE ROLE %s WITH SUPERUSER NOLOGIN;", pqIdent(testRole))
	if _, err := runPSQLAuth(user, password, "postgres", createSQL); err != nil {
		return fmt.Errorf(
			"недостаточно прав: для установки Киберстаб нужен пользователь PostgreSQL с правами суперпользователя (создание ролей с SUPERUSER)",
		)
	}
	_, _ = runPSQLAuth(user, password, "postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", pqIdent(testRole)))
	return nil
}

func OkidociDatabaseExists(user, password string) (bool, error) {
	out, err := runPSQLAuth(normalizePgUser(user), password, "postgres", "SELECT 1 FROM pg_database WHERE datname='okidoci_db';")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "1"), nil
}

// DumpDatabaseSQL creates a plain SQL dump of the given database using pg_dump.
// It writes dump to outputPath.
func DumpDatabaseSQL(user, password string, dbName string, outputPath string) error {
	user = normalizePgUser(user)
	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return fmt.Errorf("PostgreSQL not found")
	}
	pgDump := filepath.Join(info.BinDir, "pg_dump")
	if runtime.GOOS == "windows" {
		pgDump += ".exe"
	}
	if _, err := os.Stat(pgDump); err != nil {
		return fmt.Errorf("pg_dump not found: %s", pgDump)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Plain SQL format (-Fp) is easiest for manual restore/debugging.
	args := []string{"-U", user, "-d", dbName, "-Fp"}
	cmd := exec.Command(pgDump, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PGCONNECT_TIMEOUT=10")
	if strings.TrimSpace(password) != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	var stderr bytes.Buffer
	cmd.Stdout = f
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func DropOkidociDB(user, password string) error {
	user = normalizePgUser(user)
	log.Printf("[DROP] Starting okidoci_db cleanup...")

	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return fmt.Errorf("PostgreSQL not found: %v", err)
	}

	psql := filepath.Join(info.BinDir, "psql")
	if runtime.GOOS == "windows" {
		psql += ".exe"
	}

	// Helper to run psql with proper env
	runPsql := func(dbName, sql string) error {
		cmd := exec.Command(psql, "-U", user, "-d", dbName, "-c", sql)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		log.Printf("[PSQL] Running on %s: %s", dbName, sql)

		err := cmd.Run()
		if err != nil {
			log.Printf("[PSQL] ERROR: %v", err)
			if stderr.Len() > 0 {
				log.Printf("[PSQL] stderr: %s", strings.TrimSpace(stderr.String()))
			}
			return err
		}

		if stdout.Len() > 0 {
			log.Printf("[PSQL] stdout: %s", strings.TrimSpace(stdout.String()))
		}
		return nil
	}

	// Step 1: Delete sec_user data (ignore if table doesn't exist)
	log.Printf("[DROP] Step 1: Deleting sec_user data...")
	if err := runPsql("okidoci_db", "DELETE FROM sec_user;"); err != nil {
		log.Printf("[DROP] Warning: DELETE FROM sec_user failed (table may not exist): %v", err)
	}

	// Step 2: Terminate all connections to okidoci_db
	log.Printf("[DROP] Step 2: Terminating connections...")
	_ = runPsql("postgres", "ALTER DATABASE okidoci_db CONNECTION LIMIT 0;")
	if err := runPsql("postgres", "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='okidoci_db' AND pid <> pg_backend_pid();"); err != nil {
		log.Printf("[DROP] Warning: pg_terminate_backend failed: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Step 3: Drop specific roles in order
	log.Printf("[DROP] Step 3: Dropping roles...")
	roles := []string{"okidoci_admin", "okidoci_service_user_name", "okidoci_users"}
	for _, role := range roles {
		sql := fmt.Sprintf("DROP ROLE IF EXISTS %s;", role)
		if err := runPsql("postgres", sql); err != nil {
			log.Printf("[DROP] Warning: Failed to drop role %s: %v", role, err)
		}
	}

	// Drop remaining okidoci_* roles
	if err := runPsql("postgres", "DO $$ DECLARE r record; BEGIN FOR r IN SELECT rolname FROM pg_roles WHERE rolname LIKE 'okidoci_%' LOOP BEGIN EXECUTE format('DROP ROLE IF EXISTS %I', r.rolname); EXCEPTION WHEN others THEN NULL; END; END LOOP; END $$;"); err != nil {
		log.Printf("[DROP] Warning: Failed to drop remaining okidoci_* roles: %v", err)
	}

	// Drop oki_* roles
	if err := runPsql("postgres", "DO $$ DECLARE r record; BEGIN FOR r IN SELECT rolname FROM pg_roles WHERE rolname LIKE 'oki_%' LOOP BEGIN EXECUTE format('DROP ROLE IF EXISTS %I', r.rolname); EXCEPTION WHEN others THEN NULL; END; END LOOP; END $$;"); err != nil {
		log.Printf("[DROP] Warning: Failed to drop oki_* roles: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Step 4: Drop the database
	log.Printf("[DROP] Step 4: Dropping database...")
	if err := runPsql("postgres", "DROP DATABASE IF EXISTS okidoci_db;"); err != nil {
		return fmt.Errorf("failed to drop okidoci_db: %w", err)
	}

	log.Printf("[DROP] okidoci_db dropped successfully")
	return nil
}

// runPSQLWithLog runs a psql command with logging
func runPSQLWithLog(user, password, dbName, sql string) error {
	user = normalizePgUser(user)
	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return fmt.Errorf("PostgreSQL not found")
	}
	psql := filepath.Join(info.BinDir, "psql")
	if runtime.GOOS == "windows" {
		psql += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"-U", user, "-d", dbName, "-c", sql}
	cmd := exec.CommandContext(ctx, psql, args...)
	cmd.Env = os.Environ()
	if strings.TrimSpace(password) != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("[PSQL] Error executing SQL on %s: %v", dbName, err)
		if stderr.Len() > 0 {
			log.Printf("[PSQL] stderr: %s", stderr.String())
		}
		return err
	}
	
	log.Printf("[PSQL] OK: %s on %s", sql, dbName)
	return nil
}

// runPSQLQuiet runs a psql command silently (best-effort, no output)
func runPSQLQuiet(user, password, dbName, sql string) (string, error) {
	user = normalizePgUser(user)
	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return "", fmt.Errorf("PostgreSQL not found")
	}
	psql := filepath.Join(info.BinDir, "psql")
	if runtime.GOOS == "windows" {
		psql += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"-U", user, "-d", dbName, "-c", sql}
	cmd := exec.CommandContext(ctx, psql, args...)
	cmd.Env = os.Environ()
	if strings.TrimSpace(password) != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}

	// Discard all output
	cmd.Stdout = nil
	cmd.Stderr = nil

	return "", cmd.Run()
}

func DropOkiUserRoles(user, password string) error {
	return DropCyberstabRoles(user, password)
}

// DropCyberstabRoles removes all roles that belong to Cyberstab:
// - okidoci_* (including fixed roles)
// - oki_*
func DropCyberstabRoles(user, password string) error {
	user = normalizePgUser(user)
	// Drop oki_* (dynamic) first, then okidoci_* (fixed).
	_, _ = runPSQLQuiet(user, password, "postgres", `
DO $$ DECLARE r record;
BEGIN
  FOR r IN
    SELECT rolname FROM pg_roles WHERE rolname LIKE 'oki\_%' ESCAPE '\'
  LOOP
    BEGIN
      EXECUTE format('DROP ROLE IF EXISTS %I', r.rolname);
    EXCEPTION WHEN others THEN
      -- best-effort: keep going
    END;
  END LOOP;
END $$;`)

	_, err := runPSQLQuiet(user, password, "postgres", `
DO $$ DECLARE r record;
BEGIN
  FOR r IN
    SELECT rolname FROM pg_roles WHERE rolname LIKE 'okidoci\_%' ESCAPE '\'
  LOOP
    BEGIN
      EXECUTE format('DROP ROLE IF EXISTS %I', r.rolname);
    EXCEPTION WHEN others THEN
      -- best-effort: keep going
    END;
  END LOOP;
END $$;`)
	return err
}

func SetUserPassword(username, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("укажите пользователя PostgreSQL")
	}
	if newPassword == "" {
		return errors.New("новый пароль не должен быть пустым")
	}
	sql := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s';", pqIdent(username), strings.ReplaceAll(newPassword, "'", "''"))
	// Local trust auth as postgres is used to reset any role password on Windows.
	_, err := runPSQLAuth("postgres", "", "postgres", sql)
	if err != nil {
		return fmt.Errorf("не удалось сменить пароль для %s (запустите установщик от администратора): %w", username, err)
	}
	return nil
}

func StartPostgresServiceBestEffort() {
	// Best-effort and intentionally quiet.
	// Real implementation depends on how PostgreSQL is installed (service name varies).
}

func runPSQLAuth(user, password, dbName, sql string) (string, error) {
	user = normalizePgUser(user)
	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return "", fmt.Errorf("PostgreSQL not found")
	}
	psql := filepath.Join(info.BinDir, "psql")
	if runtime.GOOS == "windows" {
		psql += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	args := []string{"-U", user, "-d", dbName, "-t", "-A", "-c", sql}
	cmd := exec.CommandContext(ctx, psql, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	cmd.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=5")
	if password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		msg := decodePSQLOutput(stderr.Bytes())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

var (
	reQuotedRole = regexp.MustCompile(`(?i)(?:role|роль)\s+"([^"]+)"`)
	reQuotedUser = regexp.MustCompile(`(?i)(?:for user|для пользователя)\s+"([^"]+)"`)
)

func decodePSQLOutput(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return strings.TrimSpace(string(b))
	}
	if runtime.GOOS == "windows" {
		for _, cm := range []*charmap.Charmap{charmap.CodePage866, charmap.Windows1251} {
			if s, err := cm.NewDecoder().Bytes(b); err == nil && utf8.Valid(s) {
				return strings.TrimSpace(string(s))
			}
		}
	}
	return strings.TrimSpace(string(b))
}

func dbEngineLabel() string {
	if ae := GetActiveEngine(); strings.TrimSpace(ae.DisplayName) != "" {
		return ae.DisplayName
	}
	return "СУБД"
}

func friendlyCredentialError(user, raw string) error {
	text := strings.TrimSpace(raw)
	lower := strings.ToLower(text)

	role := extractQuotedRole(text)
	if role == "" {
		role = user
	}

	engine := dbEngineLabel()

	switch {
	case strings.Contains(lower, "password authentication failed"),
		strings.Contains(lower, "authentication failed"),
		strings.Contains(lower, "неверный пароль"),
		strings.Contains(lower, "проверка подлинности") && strings.Contains(lower, "парол"):
		return fmt.Errorf("неверный пароль пользователя «%s»", role)

	case strings.Contains(lower, "does not exist") && (strings.Contains(lower, "role") || strings.Contains(lower, "роль")),
		strings.Contains(lower, "не существует") && (strings.Contains(lower, "role") || strings.Contains(lower, "роль")):
		return fmt.Errorf("пользователь «%s» не существует", role)

	case strings.Contains(lower, "is not permitted to log in"),
		strings.Contains(lower, "запрещ") && strings.Contains(lower, "вход"):
		return fmt.Errorf("пользователю «%s» запрещён вход в %s", role, engine)

	case strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "could not connect"),
		strings.Contains(lower, "не удается подключиться"),
		strings.Contains(lower, "не удаётся подключиться"),
		strings.Contains(lower, "отклонил") && strings.Contains(lower, "подключ"):
		return fmt.Errorf("не удалось подключиться к %s: сервис не запущен или недоступен", engine)

	case strings.Contains(lower, "timeout"),
		strings.Contains(lower, "timed out"),
		strings.Contains(lower, "время ожидания"):
		return fmt.Errorf("превышено время ожидания подключения к %s", engine)

	case strings.Contains(lower, "not found") && strings.Contains(lower, "postgresql"):
		return fmt.Errorf("%s не найдена", engine)
	}

	if strings.Contains(lower, "psql:") || strings.Contains(text, "\ufffd") {
		return fmt.Errorf("не удалось подключиться к %s: проверьте имя пользователя и пароль", engine)
	}

	return fmt.Errorf("не удалось подключиться к %s: %s", engine, text)
}

func extractQuotedRole(s string) string {
	if m := reQuotedRole.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	if m := reQuotedUser.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}

func discoverPostgresBin() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("unsupported OS")
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "PostgreSQL"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "PostgreSQL"),
		`C:\Program Files\PostgreSQL`,
	}
	for _, base := range candidates {
		if strings.TrimSpace(base) == "" || base == "PostgreSQL" {
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
			bin := filepath.Join(base, e.Name(), "bin")
			psql := filepath.Join(bin, "psql.exe")
			if _, err := os.Stat(psql); err == nil {
				return bin, nil
			}
		}
	}
	return "", errors.New("PostgreSQL not found")
}

func discoverJatobaBin() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("unsupported OS")
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "GIS", "Jatoba"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "GIS", "Jatoba"),
		`C:\Program Files\GIS\Jatoba`,
	}
	for _, base := range candidates {
		if strings.TrimSpace(base) == "" || base == "Jatoba" {
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
			bin := filepath.Join(base, e.Name(), "bin")
			psql := filepath.Join(bin, "psql.exe")
			if _, err := os.Stat(psql); err == nil {
				return bin, nil
			}
		}
	}
	return "", errors.New("Jatoba not found")
}

func pqIdent(s string) string {
	// Very small helper: identifiers should be safe ASCII in our use case.
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}


