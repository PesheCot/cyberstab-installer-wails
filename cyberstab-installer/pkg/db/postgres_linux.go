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
		"/usr/pgsql",
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
		"postgresql@14-main",
		"postgresql@15-main",
		"postgresql@16-main",
		"postgresql@17-main",
	} {
		cmd := exec.Command("systemctl", "start", unit)
		_ = cmd.Run()
	}
}

// runPSQLAsLocalSuperuser connects via peer/trust as the postgres OS user.
// When the installer runs under sudo/root, psql must not use -U postgres as root.
func runPSQLAsLocalSuperuser(dbName, sql string) (string, error) {
	info, err := CheckPostgres()
	if err != nil || info == nil || !info.Installed {
		return "", fmt.Errorf("PostgreSQL not found")
	}
	psql := filepath.Join(info.BinDir, "psql")
	psqlArgs := []string{"-d", dbName, "-t", "-A", "-c", sql}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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

	cmd.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=5")
	hideCmd(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := decodePSQLOutput(stderr.Bytes())
		if msg == "" {
			msg = err.Error()
		}
		if os.Geteuid() == 0 {
			return "", fmt.Errorf("%s (нужен локальный peer-доступ для пользователя postgres)", msg)
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
