//go:build linux

package db

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
