//go:build linux

package installer

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	pgPkgAltRe    = regexp.MustCompile(`^postgresql(\d+)-server$`)
	pgPkgDebianRe = regexp.MustCompile(`^postgresql-(\d+)$`)
	pgVerNumRe    = regexp.MustCompile(`(\d{1,2}(?:\.\d+)?)`)
)

// DetectLinuxOS identifies the distribution for package-manager PostgreSQL install.
func DetectLinuxOS() (LinuxOSInfo, error) {
	if b, err := os.ReadFile("/etc/altlinux-release"); err == nil {
		text := strings.ToLower(string(b))
		info := LinuxOSInfo{Type: "ALT"}
		if strings.Contains(text, "workstation") {
			info.Edition = "workstation"
		} else if strings.Contains(text, "server") {
			info.Edition = "server"
		}
		if m := pgVerNumRe.FindString(string(b)); m != "" {
			info.Version = m
		}
		return info, nil
	}
	if b, err := os.ReadFile("/etc/astra_version"); err == nil {
		info := LinuxOSInfo{Type: "ASTRA", Version: strings.TrimSpace(string(b))}
		if rel, err := os.ReadFile("/etc/os-release"); err == nil {
			relLower := strings.ToLower(string(rel))
			switch {
			case strings.Contains(relLower, "orel"):
				info.Edition = "common"
			case strings.Contains(relLower, "smolensk"), strings.Contains(relLower, "voronezh"), strings.Contains(relLower, "special"):
				info.Edition = "se"
			default:
				if strings.HasPrefix(info.Version, "1.7") || strings.HasPrefix(info.Version, "1.8") {
					info.Edition = "se"
				}
			}
		}
		return info, nil
	}
	if b, err := os.ReadFile("/etc/redos-release"); err == nil {
		info := LinuxOSInfo{Type: "REDOS"}
		if m := pgVerNumRe.FindString(string(b)); m != "" {
			info.Version = m
		}
		return info, nil
	}
	if info, ok := detectFromOSRelease(); ok {
		return info, nil
	}
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		if b, err := os.ReadFile("/etc/debian_version"); err == nil {
			ver := strings.TrimSpace(strings.Split(string(b), ".")[0])
			return LinuxOSInfo{Type: "DEBIAN", Version: ver}, nil
		}
	}
	return LinuxOSInfo{}, fmt.Errorf("ОС не распознана для установки PostgreSQL из пакетов")
}

func detectFromOSRelease() (LinuxOSInfo, bool) {
	vals := parseOSRelease()
	if len(vals) == 0 {
		return LinuxOSInfo{}, false
	}
	id := strings.ToLower(strings.TrimSpace(vals["ID"]))
	name := strings.ToLower(strings.TrimSpace(vals["NAME"]))
	version := strings.TrimSpace(vals["VERSION_ID"])
	switch {
	case id == "ubuntu":
		return LinuxOSInfo{Type: "UBUNTU", Version: version}, true
	case id == "osnova" || strings.Contains(name, "основа") || strings.Contains(name, "osnova"):
		return LinuxOSInfo{Type: "OSNOVA", Version: version}, true
	case id == "debian":
		return LinuxOSInfo{Type: "DEBIAN", Version: version}, true
	case strings.Contains(strings.ToLower(vals["ID_LIKE"]), "debian"):
		return LinuxOSInfo{Type: "DEBIAN", Version: version}, true
	}
	return LinuxOSInfo{}, false
}

func parseOSRelease() map[string]string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[k] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return out
}

// FindAvailablePostgresPackages queries apt/dnf for installable PostgreSQL versions.
func FindAvailablePostgresPackages() ([]PostgresPackageOption, error) {
	osInfo, err := DetectLinuxOS()
	if err != nil {
		return nil, err
	}
	log.Printf("[PG-PKG] OS: %s %s %s", osInfo.Type, osInfo.Edition, osInfo.Version)

	var versions []string
	switch osInfo.Type {
	case "ALT":
		versions = aptCachePostgresVersions(`^postgresql[0-9][0-9]-server$`, pgPkgAltRe, 1)
	case "ASTRA", "DEBIAN", "UBUNTU", "OSNOVA":
		versions = aptCachePostgresVersions(`^postgresql-[0-9]+$`, pgPkgDebianRe, 1)
	case "REDOS":
		versions = dnfPostgresVersions()
	default:
		return nil, fmt.Errorf("неподдерживаемая ОС: %s", osInfo.Type)
	}

	if len(versions) == 0 {
		versions = []string{"17", "16", "15", "14", "13"}
		log.Printf("[PG-PKG] WARN: versions from repos not found, using defaults")
	}

	seen := map[string]bool{}
	var out []PostgresPackageOption
	for _, v := range versions {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		if !repoHasPostgresPackage(osInfo, v) {
			continue
		}
		seen[v] = true
		out = append(out, PostgresPackageOption{
			Version:     v,
			Label:       fmt.Sprintf("PostgreSQL %s", v),
			PackageName: postgresPackageName(osInfo, v),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		vi, _ := strconv.ParseFloat(out[i].Version, 64)
		vj, _ := strconv.ParseFloat(out[j].Version, 64)
		return vi > vj
	})
	return out, nil
}

func postgresPackageName(osInfo LinuxOSInfo, version string) string {
	switch osInfo.Type {
	case "ALT":
		return "postgresql" + version + "-server"
	default:
		return "postgresql-" + version
	}
}

func repoHasPostgresPackage(osInfo LinuxOSInfo, version string) bool {
	switch osInfo.Type {
	case "REDOS":
		if _, err := exec.LookPath("dnf"); err != nil {
			return false
		}
		return exec.Command("dnf", "list", "postgresql-server").Run() == nil
	default:
		if _, err := exec.LookPath("apt-cache"); err != nil {
			return false
		}
		pkg := postgresPackageName(osInfo, version)
		return exec.Command("apt-cache", "show", pkg).Run() == nil
	}
}

func aptCachePostgresVersions(searchPattern string, re *regexp.Regexp, group int) []string {
	if _, err := exec.LookPath("apt-cache"); err != nil {
		return nil
	}
	out, err := exec.Command("apt-cache", "search", searchPattern).Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var versions []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if m := re.FindStringSubmatch(fields[0]); len(m) > group {
			seen[m[group]] = true
		}
	}
	for v := range seen {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		vi, _ := strconv.Atoi(versions[i])
		vj, _ := strconv.Atoi(versions[j])
		return vi > vj
	})
	return versions
}

func dnfPostgresVersions() []string {
	if _, err := exec.LookPath("dnf"); err != nil {
		return nil
	}
	out, err := exec.Command("dnf", "list", "postgresql-server").Output()
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "postgresql-server") {
			if m := pgVerNumRe.FindString(line); m != "" {
				return []string{strings.Split(m, ".")[0]}
			}
		}
	}
	return nil
}

// InstallPostgresPackage installs PostgreSQL from distro packages (apt/dnf).
func InstallPostgresPackage(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("не указана версия PostgreSQL")
	}
	osInfo, err := DetectLinuxOS()
	if err != nil {
		return err
	}
	log.Printf("[PG-PKG] Installing PostgreSQL %s on %s", version, osInfo.Type)

	switch osInfo.Type {
	case "ALT":
		if err := runPackageCmd("apt-get", "update"); err != nil {
			return err
		}
		pkg := "postgresql" + version + "-server"
		if err := runPackageCmd("apt-get", "install", "-y", pkg); err != nil {
			return err
		}
		if err := altInitdbIfNeeded(); err != nil {
			log.Printf("[PG-PKG] WARN: initdb/service: %v", err)
		}
	case "ASTRA", "DEBIAN", "UBUNTU", "OSNOVA":
		if err := runPackageCmd("apt-get", "update"); err != nil {
			return err
		}
		pkg := "postgresql-" + version
		show := exec.Command("apt-cache", "show", pkg)
		if show.Run() == nil {
			if err := runPackageCmd("apt-get", "install", "-y", pkg); err != nil {
				return err
			}
		} else {
			if err := runPackageCmd("apt-get", "install", "-y", "postgresql"); err != nil {
				return err
			}
		}
		if err := enableAndStartPostgresService(); err != nil {
			log.Printf("[PG-PKG] WARN: service start: %v", err)
		}
	case "REDOS":
		if err := runPackageCmd("dnf", "install", "-y", "postgresql-server", "postgresql"); err != nil {
			return err
		}
		if err := redosInitdbIfNeeded(); err != nil {
			log.Printf("[PG-PKG] WARN: initdb/service: %v", err)
		}
	default:
		return fmt.Errorf("установка PostgreSQL из пакетов не поддерживается для %s", osInfo.Type)
	}

	if err := ConfigureLinuxPlatform(); err != nil {
		log.Printf("[PG-PKG] WARN: OS configure: %v", err)
	}
	return nil
}

func runPackageCmd(name string, args ...string) error {
	log.Printf("[PG-PKG] %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func altInitdbIfNeeded() error {
	pgData := "/var/lib/pgsql/data"
	if st, err := os.Stat(pgData); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(pgData)
		if len(entries) > 0 {
			log.Printf("[PG-PKG] data dir %s already exists, skip initdb", pgData)
			return enableAndStartPostgresService()
		}
	}
	_ = os.MkdirAll(pgData, 0700)
	_ = exec.Command("chown", "postgres:postgres", pgData).Run()
	if st, err := os.Stat("/etc/init.d/postgresql"); err == nil && !st.IsDir() {
		return runPackageCmd("/etc/init.d/postgresql", "initdb")
	}
	return enableAndStartPostgresService()
}

func redosInitdbIfNeeded() error {
	pgData := "/var/lib/pgsql/data"
	if st, err := os.Stat(pgData); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(pgData)
		if len(entries) > 0 {
			return enableAndStartPostgresService()
		}
	}
	if _, err := exec.LookPath("postgresql-setup"); err == nil {
		return runPackageCmd("postgresql-setup", "--initdb", "--unit", "postgresql")
	}
	return enableAndStartPostgresService()
}

func enableAndStartPostgresService() error {
	units := []string{
		"postgresql.service",
		"postgresql@16-main.service",
		"postgresql@17-main.service",
		"postgresql@15-main.service",
		"postgresql@14-main.service",
		"postgresql",
	}
	for _, unit := range units {
		_ = exec.Command("systemctl", "enable", unit).Run()
		if err := exec.Command("systemctl", "start", unit).Run(); err == nil {
			log.Printf("[PG-PKG] started %s", unit)
			return nil
		}
	}
	return fmt.Errorf("не удалось запустить службу PostgreSQL")
}

// IsPackageManagerPostgresInstall reports whether selection is a package version (not a file path).
func IsPackageManagerPostgresInstall(selection string) bool {
	selection = strings.TrimSpace(selection)
	if selection == "" {
		return false
	}
	if strings.Contains(selection, string(filepath.Separator)) {
		return false
	}
	if strings.HasSuffix(strings.ToLower(selection), ".deb") ||
		strings.HasSuffix(strings.ToLower(selection), ".rpm") ||
		strings.HasSuffix(strings.ToLower(selection), ".run") ||
		strings.HasSuffix(strings.ToLower(selection), ".exe") {
		return false
	}
	return pgVerNumRe.MatchString(selection)
}
