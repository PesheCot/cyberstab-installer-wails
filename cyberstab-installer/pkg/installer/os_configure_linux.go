//go:build linux

package installer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	pgHbaPeerRe       = regexp.MustCompile(`^(\s*local\s+all\s+all\s+)peer(\s*)$`)
	pgHbaPostgresPeer = regexp.MustCompile(`^(\s*local\s+all\s+postgres\s+)peer(\s*)$`)
	pgHbaHostIdentRe  = regexp.MustCompile(`^(\s*host\s+all\s+all\s+127\.0\.0\.1/32\s+)ident(\s*)$`)
	pgHbaHostScramRe  = regexp.MustCompile(`^(\s*host\s+all\s+all\s+127\.0\.0\.1/32\s+)scram-sha-256(\s*)$`)
)

// ConfigureLinuxPlatform applies OS-specific tweaks from InstallServer.sh (libusb, pg_hba, parsec).
func ConfigureLinuxPlatform() error {
	osInfo, err := DetectLinuxOS()
	if err != nil {
		log.Printf("[OS-CFG] skip: %v", err)
		return nil
	}
	log.Printf("[OS-CFG] configuring for %s %s %s", osInfo.Type, osInfo.Edition, osInfo.Version)

	switch osInfo.Type {
	case "ALT":
		return configureALT(osInfo)
	case "ASTRA":
		return configureAstra(osInfo)
	case "REDOS":
		return configureREDOS(osInfo)
	case "DEBIAN":
		return configureDebian()
	default:
		return nil
	}
}

func configureALT(osInfo LinuxOSInfo) error {
	if osInfo.Edition == "server" {
		if err := aptInstallPackage("libusb-compat"); err != nil {
			log.Printf("[OS-CFG] WARN: libusb-compat: %v", err)
		}
	}
	return configurePgHba()
}

func configureAstra(osInfo LinuxOSInfo) error {
	switch osInfo.Edition {
	case "common":
		return configurePgHba()
	case "se":
		if astraSENeedsLibusb(osInfo.Version) {
			if err := aptInstallPackage("libusb-0.1-4"); err != nil {
				log.Printf("[OS-CFG] WARN: libusb-0.1-4: %v", err)
			}
		}
		if err := configurePgHba(); err != nil {
			log.Printf("[OS-CFG] WARN: pg_hba: %v", err)
		}
		return configureParsecMswitch()
	default:
		if strings.HasPrefix(osInfo.Version, "1.7") || strings.HasPrefix(osInfo.Version, "1.8") {
			if err := aptInstallPackage("libusb-0.1-4"); err != nil {
				log.Printf("[OS-CFG] WARN: libusb-0.1-4: %v", err)
			}
			if err := configurePgHba(); err != nil {
				log.Printf("[OS-CFG] WARN: pg_hba: %v", err)
			}
			return configureParsecMswitch()
		}
		return configurePgHba()
	}
}

func configureREDOS(osInfo LinuxOSInfo) error {
	ver := strings.TrimSpace(osInfo.Version)
	if ver == "7.3" || strings.HasPrefix(ver, "7.3") {
		if err := runLinuxCmdLogged("dnf", "install", "-y", "libusb.x86_64"); err != nil {
			log.Printf("[OS-CFG] WARN: libusb: %v", err)
		}
	}
	return configurePgHba()
}

func configureDebian() error {
	if err := aptInstallPackage("libusb-0.1-4"); err != nil {
		log.Printf("[OS-CFG] WARN: libusb-0.1-4: %v", err)
	}
	return configurePgHba()
}

func astraSENeedsLibusb(version string) bool {
	return strings.HasPrefix(strings.TrimSpace(version), "1.7") ||
		strings.HasPrefix(strings.TrimSpace(version), "1.8")
}

func aptInstallPackage(pkg string) error {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("apt-get not found")
	}
	_ = runLinuxCmdLogged("apt-get", "update")
	return runLinuxCmdLogged("apt-get", "install", "-y", pkg)
}

func configureParsecMswitch() error {
	const path = "/etc/parsec/mswitch.conf"
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[OS-CFG] %s not found, skip", path)
			return nil
		}
		return err
	}
	text := string(b)
	if strings.Contains(text, "zero_if_notfound") {
		lines := strings.Split(text, "\n")
		changed := false
		for i, line := range lines {
			if strings.Contains(line, "zero_if_notfound") {
				lines[i] = "zero_if_notfound = yes"
				changed = true
			}
		}
		if !changed {
			return nil
		}
		text = strings.Join(lines, "\n")
	} else {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "zero_if_notfound = yes\n"
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return fmt.Errorf("mswitch.conf: %w", err)
	}
	log.Printf("[OS-CFG] mswitch.conf updated")
	return nil
}

func configurePgHba() error {
	hbaPath, err := findPgHbaConf()
	if err != nil {
		return err
	}
	if hbaPath == "" {
		log.Printf("[OS-CFG] pg_hba.conf not found, skip")
		return nil
	}

	raw, err := os.ReadFile(hbaPath)
	if err != nil {
		return fmt.Errorf("read pg_hba: %w", err)
	}
	backup := hbaPath + ".backup_configure"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		if err := os.WriteFile(backup, raw, 0600); err != nil {
			log.Printf("[OS-CFG] WARN: pg_hba backup: %v", err)
		} else {
			log.Printf("[OS-CFG] pg_hba backup: %s", backup)
		}
	}

	lines := strings.Split(string(raw), "\n")
	changed := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		newLine := line
		switch {
		case pgHbaPeerRe.MatchString(line):
			newLine = pgHbaPeerRe.ReplaceAllString(line, `${1}md5${2}`)
			changed = true
		case pgHbaPostgresPeer.MatchString(line):
			newLine = pgHbaPostgresPeer.ReplaceAllString(line, `${1}md5${2}`)
			changed = true
		case pgHbaHostIdentRe.MatchString(line):
			newLine = pgHbaHostIdentRe.ReplaceAllString(line, `${1}md5${2}`)
			changed = true
		case pgHbaHostScramRe.MatchString(line):
			newLine = pgHbaHostScramRe.ReplaceAllString(line, `${1}md5${2}`)
			changed = true
		}
		lines[i] = newLine
	}
	if !changed {
		log.Printf("[OS-CFG] pg_hba already configured: %s", hbaPath)
		return restartPostgresServiceBestEffort()
	}

	out := strings.Join(lines, "\n")
	if err := os.WriteFile(hbaPath, []byte(out), 0600); err != nil {
		return fmt.Errorf("write pg_hba: %w", err)
	}
	fixPgDataPermissions(hbaPath)
	log.Printf("[OS-CFG] pg_hba configured: %s", hbaPath)
	return restartPostgresServiceBestEffort()
}

func findPgHbaConf() (string, error) {
	candidates := []string{
		"/var/lib/pgsql/data/pg_hba.conf",
		"/var/lib/postgres/data/pg_hba.conf",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	if matches, _ := filepath.Glob("/etc/postgresql/*/main/pg_hba.conf"); len(matches) > 0 {
		return matches[0], nil
	}
	if matches, _ := filepath.Glob("/etc/postgresql/*/*/pg_hba.conf"); len(matches) > 0 {
		return matches[0], nil
	}
	return "", nil
}

func fixPgDataPermissions(hbaPath string) {
	_ = exec.Command("chown", "postgres:postgres", hbaPath).Run()
	_ = os.Chmod(hbaPath, 0600)
	dir := filepath.Dir(hbaPath)
	_ = exec.Command("chown", "postgres:postgres", dir).Run()
	_ = os.Chmod(dir, 0700)
}

func restartPostgresServiceBestEffort() error {
	units := []string{
		"postgresql",
		"postgresql.service",
		"postgresql@16-main.service",
		"postgresql@17-main.service",
		"postgresql@15-main.service",
		"postgresql@14-main.service",
	}
	for _, unit := range units {
		cmd := exec.Command("systemctl", "restart", unit)
		if err := cmd.Run(); err == nil {
			log.Printf("[OS-CFG] restarted %s", unit)
			time.Sleep(2 * time.Second)
			return nil
		}
	}
	log.Printf("[OS-CFG] WARN: could not restart PostgreSQL service")
	return nil
}

func runLinuxCmdLogged(name string, args ...string) error {
	log.Printf("[OS-CFG] %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func ensureClientLogDirsLinux(installDir string) {
	for _, client := range []string{"CyberstabClientLinux64", "CyberstabClientLinux32"} {
		logDir := filepath.Join(installDirOrDefault(installDir), client, "log", "client")
		if err := os.MkdirAll(logDir, 01777); err != nil {
			log.Printf("[INSTALL] WARN: client log dir %s: %v", logDir, err)
			continue
		}
		log.Printf("[INSTALL] client log dir: %s (1777)", logDir)
	}
}
