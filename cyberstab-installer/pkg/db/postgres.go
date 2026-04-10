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
	"runtime"
	"strings"
	"syscall"
	"time"
)

type PostgresInfo struct {
	Installed bool
	BinDir    string
}

var postgresBinDir string

func SetPostgresBinDir(dir string) {
	postgresBinDir = dir
}

func CheckPostgres() (*PostgresInfo, error) {
	bin := postgresBinDir
	if strings.TrimSpace(bin) == "" {
		var err error
		bin, err = discoverPostgresBin()
		if err != nil {
			return &PostgresInfo{Installed: false}, nil
		}
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

func VerifyPassword(password string) error {
	_, err := runPSQL(password, "postgres", "SELECT 1;")
	return err
}

func OkidociDatabaseExists(password string) (bool, error) {
	out, err := runPSQL(password, "postgres", "SELECT 1 FROM pg_database WHERE datname='okidoci_db';")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "1"), nil
}

// DumpDatabaseSQL creates a plain SQL dump of the given database using pg_dump.
// It writes dump to outputPath.
func DumpDatabaseSQL(password string, dbName string, outputPath string) error {
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
	args := []string{"-U", "postgres", "-d", dbName, "-Fp"}
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
		return fmt.Errorf(msg)
	}
	return nil
}

func DropOkidociDB(password string) error {
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
		cmd := exec.Command(psql, "-U", "postgres", "-d", dbName, "-c", sql)
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
func runPSQLWithLog(password, dbName, sql string) error {
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

	args := []string{"-U", "postgres", "-d", dbName, "-c", sql}
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
func runPSQLQuiet(password, dbName, sql string) (string, error) {
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

	args := []string{"-U", "postgres", "-d", dbName, "-c", sql}
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

func DropOkiUserRoles(password string) error {
	return DropCyberstabRoles(password)
}

// DropCyberstabRoles removes all roles that belong to Cyberstab:
// - okidoci_* (including fixed roles)
// - oki_*
func DropCyberstabRoles(password string) error {
	// Drop oki_* (dynamic) first, then okidoci_* (fixed).
	_, _ = runPSQLQuiet(password, "postgres", `
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

	_, err := runPSQLQuiet(password, "postgres", `
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
	if strings.TrimSpace(username) == "" {
		return errors.New("username is required")
	}
	if newPassword == "" {
		return errors.New("password is required")
	}
	sql := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s';", pqIdent(username), strings.ReplaceAll(newPassword, "'", "''"))
	_, err := runPSQL("", "postgres", sql)
	return err
}

func StartPostgresServiceBestEffort() {
	// Best-effort and intentionally quiet.
	// Real implementation depends on how PostgreSQL is installed (service name varies).
}

func runPSQL(password, dbName, sql string) (string, error) {
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

	args := []string{"-U", "postgres", "-d", dbName, "-t", "-A", "-c", sql}
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
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf(msg)
	}
	return stdout.String(), nil
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

func pqIdent(s string) string {
	// Very small helper: identifiers should be safe ASCII in our use case.
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}


