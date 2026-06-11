package installer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type PostgresInstaller struct {
	Path    string
	Label   string
	Version string
}

var pgVersionInNameRe = regexp.MustCompile(`(?i)(?:postgres(?:ql|pro)?|pgpro|jatoba)[^\d]*(\d{1,2}(?:\.\d+)?)|(\d{1,2}(?:\.\d+)?)[^\d]*(?:postgres|pgpro)`)

// FindPostgresInstallers scans search roots for PostgreSQL / Postgres Pro / Jatoba installers.
func FindPostgresInstallers(searchRoots []string) []PostgresInstaller {
	seen := map[string]bool{}
	var out []PostgresInstaller
	for _, root := range searchRoots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				rel, _ := filepath.Rel(root, path)
				if rel != "." && strings.Count(rel, string(os.PathSeparator)) > 4 {
					return filepath.SkipDir
				}
				return nil
			}
			if !isPostgresInstallerFile(d.Name()) {
				return nil
			}
			key := strings.ToLower(filepath.Clean(path))
			if seen[key] {
				return nil
			}
			seen[key] = true
			out = append(out, PostgresInstaller{
				Path:    path,
				Label:   postgresInstallerLabel(path),
				Version: postgresInstallerVersion(d.Name()),
			})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Version != out[j].Version {
			return out[i].Version > out[j].Version
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func isPostgresInstallerFile(name string) bool {
	lower := strings.ToLower(name)
	if !(strings.Contains(lower, "postgres") || strings.Contains(lower, "pgpro") || strings.Contains(lower, "jatoba")) {
		return false
	}
	switch filepath.Ext(lower) {
	case ".exe", ".msi", ".deb", ".rpm", ".run", ".sh", ".bin":
		return true
	default:
		return strings.Contains(lower, "setup") || strings.Contains(lower, "install")
	}
}

func postgresInstallerVersion(name string) string {
	if m := pgVersionInNameRe.FindStringSubmatch(name); len(m) > 0 {
		if m[1] != "" {
			return m[1]
		}
		if len(m) > 2 && m[2] != "" {
			return m[2]
		}
	}
	if m := regexp.MustCompile(`(\d{2})`).FindStringSubmatch(name); len(m) > 1 {
		return m[1]
	}
	return ""
}

func postgresInstallerLabel(path string) string {
	name := filepath.Base(path)
	ver := postgresInstallerVersion(name)
	lower := strings.ToLower(name)
	kind := "PostgreSQL"
	switch {
	case strings.Contains(lower, "jatoba"):
		kind = "Jatoba"
	case strings.Contains(lower, "pgpro") || strings.Contains(lower, "postgrespro"):
		kind = "Postgres Pro"
	}
	if ver != "" {
		return fmt.Sprintf("%s %s", kind, ver)
	}
	return kind + " (" + name + ")"
}

// RunPostgresInstaller launches the selected installer and waits for it to finish.
func RunPostgresInstaller(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return errors.New("путь к установщику PostgreSQL не указан")
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("установщик не найден: %w", err)
	}
	if st.IsDir() {
		return fmt.Errorf("указана папка, а не файл установщика: %s", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	log.Printf("[PG-INSTALL] Running %s", path)

	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "windows" && (ext == ".exe" || ext == ".msi"):
		cmd = exec.Command(path)
		hideCmd(cmd)
	case ext == ".deb":
		if _, err := exec.LookPath("dpkg"); err == nil {
			cmd = exec.Command("dpkg", "-i", path)
		} else {
			cmd = exec.Command("apt-get", "install", "-y", path)
		}
	case ext == ".rpm":
		if _, err := exec.LookPath("dnf"); err == nil {
			cmd = exec.Command("dnf", "install", "-y", path)
		} else {
			cmd = exec.Command("rpm", "-Uvh", path)
		}
	case ext == ".run", ext == ".sh", ext == ".bin", ext == "":
		_ = os.Chmod(path, st.Mode()|0755)
		cmd = exec.Command(path)
	default:
		cmd = exec.Command(path)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("установщик PostgreSQL завершился с ошибкой: %w", err)
	}
	return nil
}
