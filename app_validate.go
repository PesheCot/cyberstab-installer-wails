package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cyberstab-installer/pkg/fs"
	installer "cyberstab-installer/pkg/installer"
)

type SourceValidationResult struct {
	Valid   bool     `json:"valid"`
	Missing []string `json:"missing"`
}

type PostgresInstallerDTO struct {
	Path    string `json:"path"`
	Label   string `json:"label"`
	Version string `json:"version"`
}

// ValidateSourceRoot checks that the path contains required Cyberstab distro folders.
func (a *App) ValidateSourceRoot(root string, wantServerOrDB, wantClients bool) (SourceValidationResult, error) {
	missing, err := installer.ValidateDistroRoot(root, wantServerOrDB, wantClients)
	if err != nil {
		return SourceValidationResult{}, err
	}
	return SourceValidationResult{
		Valid:   len(missing) == 0,
		Missing: missing,
	}, nil
}

// ListPostgresInstallers searches USB/distrib for PostgreSQL installers.
func (a *App) ListPostgresInstallers(sourceRoot string) []PostgresInstallerDTO {
	roots := postgresInstallerSearchRoots(sourceRoot)
	found := installer.FindPostgresInstallers(roots)
	dto := make([]PostgresInstallerDTO, 0, len(found))
	for _, item := range found {
		dto = append(dto, PostgresInstallerDTO{
			Path:    item.Path,
			Label:   item.Label,
			Version: item.Version,
		})
	}
	return dto
}

// InstallPostgresInstaller runs the selected PostgreSQL installer package.
func (a *App) InstallPostgresInstaller(installerPath string) error {
	return installer.RunPostgresInstaller(installerPath)
}

func postgresInstallerSearchRoots(sourceRoot string) []string {
	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" || seen[p] {
			return
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		seen[p] = true
		roots = append(roots, p)
	}

	if sourceRoot != "" {
		add(sourceRoot)
	}
	f := fs.NewFinder()
	for _, r := range f.FindDistros() {
		add(r)
	}
	if isWindows() {
		for c := byte('A'); c <= 'Z'; c++ {
			add(fmt.Sprintf("%c:\\", c))
		}
	} else {
		for _, base := range []string{"/run/media", "/media", "/mnt"} {
			add(base)
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				p1 := filepath.Join(base, e.Name())
				add(p1)
				sub, err := os.ReadDir(p1)
				if err != nil {
					continue
				}
				for _, s := range sub {
					if s.IsDir() {
						add(filepath.Join(p1, s.Name()))
					}
				}
			}
		}
	}
	return roots
}

// ValidateSqlBackupPath checks that a .sql backup file exists.
func (a *App) ValidateSqlBackupPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("укажите путь к файлу .sql")
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("файл не найден: %s", path)
	}
	if st.IsDir() {
		return fmt.Errorf("указана папка, а не файл: %s", path)
	}
	return nil
}
