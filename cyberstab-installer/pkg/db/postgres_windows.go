//go:build windows

package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func discoverPostgresBin() (string, error) {
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

func StartPostgresServiceBestEffort() {
	// Best-effort; Windows service names vary by PostgreSQL version.
}

func runPSQLAsLocalSuperuser(dbName, sql string) (string, error) {
	return runPSQLAuth("postgres", "", dbName, sql)
}

func setUserPasswordPlatform(username, newPassword string) error {
	sql := alterUserPasswordSQL(username, newPassword)
	if _, err := runPSQLAsLocalSuperuser("postgres", sql); err != nil {
		return fmt.Errorf("не удалось сменить пароль для %s (запустите установщик от администратора): %w", username, err)
	}
	return nil
}
