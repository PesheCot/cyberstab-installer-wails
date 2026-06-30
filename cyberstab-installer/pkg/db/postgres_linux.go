//go:build linux

package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func discoverPostgresBin() (string, error) {
	if bin := discoverPostgresBinFromPath(); bin != "" {
		return bin, nil
	}
	var candidates []string
	for _, base := range []string{
		"/usr/lib/postgresql",
		"/opt/postgresql",
		"/usr/local/pgsql",
	} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidates = append(candidates, filepath.Join(base, e.Name(), "bin"))
		}
	}
	candidates = append(candidates, "/usr/bin", "/usr/local/bin")
	sort.Strings(candidates)
	for _, bin := range candidates {
		if hasPsql(bin) {
			return bin, nil
		}
	}
	return "", errors.New("PostgreSQL not found")
}

func discoverJatobaBin() (string, error) {
	for _, base := range []string{
		"/opt/jatoba",
		"/usr/local/jatoba",
		"/opt/GIS/Jatoba",
	} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			bin := filepath.Join(base, e.Name(), "bin")
			if hasPsql(bin) {
				return bin, nil
			}
		}
	}
	return "", errors.New("Jatoba not found")
}

func discoverPostgresBinFromPath() string {
	out, err := exec.LookPath("psql")
	if err != nil || strings.TrimSpace(out) == "" {
		return ""
	}
	return filepath.Dir(out)
}

func hasPsql(binDir string) bool {
	psql := filepath.Join(binDir, "psql")
	st, err := os.Stat(psql)
	return err == nil && !st.IsDir()
}

func StartPostgresServiceBestEffort() {
	for _, unit := range []string{
		"postgresql",
		"postgresql@18-main",
		"postgresql@17-main",
		"postgresql@16-main",
		"postgresql@15-main",
		"postgresql@14-main",
		"postgresql@13-main",
		"postgresql@12-main",
		"postgresql@11-main",
		"postgresql@10-main",
		"postgresql@9.6-main",
		"pgpro-18",
		"pgpro-17",
		"pgpro-16",
		"pgpro-15",
		"postgrespro",
	} {
		cmd := exec.Command("systemctl", "start", unit)
		_ = cmd.Run()
	}
}

// runPSQLAsLocalSuperuser connects via peer/trust as the postgres OS user.
func runPSQLAsLocalSuperuser(dbName, sql string) (string, error) {
	return runPSQLSuperuserPeerOnly(dbName, sql)
}

// runPSQLSuperuserPeerOnly uses Unix socket (peer auth) without interactive prompts.
func runPSQLSuperuserPeerOnly(dbName, sql string) (string, error) {
	var lastErr error
	for _, mode := range []string{"socket", "tmp-socket"} {
		out, err := runPSQLAsPostgresOS(dbName, sql, mode)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isPostgresAuthError(err) {
			return "", err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("не удалось подключиться к PostgreSQL")
	}
	return "", lastErr
}

func runPSQLSuperuserMulti(dbName, sql string) (string, error) {
	var lastErr error
	for _, mode := range []string{"socket", "tcp", "tmp-socket"} {
		out, err := runPSQLAsPostgresOS(dbName, sql, mode)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("не удалось подключиться к PostgreSQL")
	}
	return "", lastErr
}

func runPSQLAsPostgresOS(dbName, sql, mode string) (string, error) {
	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return "", fmt.Errorf("PostgreSQL not found")
	}
	psql := filepath.Join(info.BinDir, "psql")
	// -w: never prompt on tty (password reset runs while user already typed the new password in CLI).
	psqlArgs := []string{"-w", "-d", dbName, "-t", "-A", "-c", sql}
	switch mode {
	case "tcp":
		psqlArgs = append([]string{"-h", "127.0.0.1"}, psqlArgs...)
	case "tmp-socket":
		psqlArgs = append([]string{"-h", "/tmp"}, psqlArgs...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch {
	case os.Geteuid() == 0:
		if runuser, lookErr := exec.LookPath("runuser"); lookErr == nil {
			cmd = exec.CommandContext(ctx, runuser, append([]string{"-u", "postgres", "--", psql}, psqlArgs...)...)
		} else if sudo, lookErr := exec.LookPath("sudo"); lookErr == nil {
			cmd = exec.CommandContext(ctx, sudo, append([]string{"-u", "postgres", psql}, psqlArgs...)...)
		} else {
			return "", fmt.Errorf("не найден runuser/sudo для смены пароля PostgreSQL")
		}
	case isOSUser("postgres"):
		cmd = exec.CommandContext(ctx, psql, psqlArgs...)
	default:
		return runPSQLAuth("postgres", "", dbName, sql)
	}

	cmd.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=8")
	cmd.Stdin = nil
	hideCmd(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := decodePSQLOutput(stderr.Bytes())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

func isOSUser(name string) bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(u.Username), name)
}
