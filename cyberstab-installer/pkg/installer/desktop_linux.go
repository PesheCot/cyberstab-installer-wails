//go:build linux

package installer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var userDirsDesktopRe = regexp.MustCompile(`(?m)^XDG_DESKTOP_DIR="([^"]+)"`)

func createClientDesktopEntryLinux(installDir string) error {
	clientDir := DetectClientDirLinux(installDir)
	if clientDir == "" {
		return fmt.Errorf("client directory not found")
	}
	targetExe := FindClientExeBestEffort(clientDir)
	if targetExe == "" {
		return fmt.Errorf("client executable not found in %s", clientDir)
	}
	_ = ensureExecutable(targetExe)

	icon := findClientIcon(clientDir)
	content := buildClientDesktopEntry(targetExe, filepath.Dir(targetExe), icon)

	if os.Geteuid() == 0 {
		systemPath := "/usr/share/applications/cyberstab-client.desktop"
		if err := writeDesktopEntry(systemPath, content, 0644, ""); err != nil {
			log.Printf("[INSTALL] WARN: system menu entry: %v", err)
		} else {
			log.Printf("[INSTALL] Client menu entry: %s", systemPath)
			_ = runLinuxCmd("update-desktop-database", "/usr/share/applications")
		}
	}

	var wrote bool
	for _, home := range linuxShortcutUserHomes() {
		owner := linuxHomeOwner(home)
		for _, desktopDir := range linuxDesktopDirs(home) {
			for _, name := range []string{"Киберстаб.desktop", "cyberstab-client.desktop"} {
				entryPath := filepath.Join(desktopDir, name)
				if err := writeDesktopEntry(entryPath, content, 0755, owner); err != nil {
					log.Printf("[INSTALL] WARN: desktop icon %s: %v", entryPath, err)
					continue
				}
				log.Printf("[INSTALL] Client desktop icon: %s", entryPath)
				wrote = true
			}
		}
	}
	if !wrote {
		return fmt.Errorf("не удалось создать ярлык на рабочем столе")
	}
	return nil
}

func linuxHomeOwner(home string) string {
	home = filepath.Clean(home)
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" && sudoUser != "root" {
		if u, err := user.Lookup(sudoUser); err == nil && filepath.Clean(u.HomeDir) == home {
			return sudoUser
		}
	}
	if u, err := user.Current(); err == nil && filepath.Clean(u.HomeDir) == home {
		return u.Username
	}
	return ""
}

func linuxShortcutUserHomes() []string {
	var homes []string
	seen := map[string]bool{}
	sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER"))

	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" || seen[h] {
			return
		}
		if st, err := os.Stat(h); err != nil || !st.IsDir() {
			return
		}
		seen[h] = true
		homes = append(homes, h)
	}

	if sudoUser != "" && sudoUser != "root" {
		if u, err := user.Lookup(sudoUser); err == nil {
			add(u.HomeDir)
		}
	}
	if u, err := user.Current(); err == nil {
		if sudoUser == "" || u.Username != "root" {
			add(u.HomeDir)
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		if sudoUser == "" || h != "/root" {
			add(h)
		}
	}
	return homes
}

func linuxDesktopDirs(home string) []string {
	home = strings.TrimSpace(home)
	if home == "" {
		return nil
	}
	var dirs []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		seen[p] = true
		dirs = append(dirs, p)
	}

	if b, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs")); err == nil {
		if m := userDirsDesktopRe.FindStringSubmatch(string(b)); len(m) == 2 {
			add(strings.ReplaceAll(m[1], "$HOME", home))
		}
	}
	for _, name := range []string{"Desktop", "Рабочий стол", "desktop"} {
		add(filepath.Join(home, name))
	}
	return dirs
}

func findClientIcon(clientDir string) string {
	var hits []string
	_ = filepath.WalkDir(clientDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".svg") || strings.HasSuffix(lower, ".xpm") {
			if strings.Contains(lower, "cyberstab") || strings.Contains(lower, "icon") || strings.Contains(lower, "logo") {
				hits = append(hits, path)
			}
		}
		return nil
	})
	if len(hits) > 0 {
		return hits[0]
	}
	return ""
}

func buildClientDesktopEntry(execPath, workDir, icon string) string {
	execPath = strings.TrimSpace(execPath)
	workDir = strings.TrimSpace(workDir)
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Version=1.0\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Name=Киберстаб\n")
	b.WriteString("Name[ru]=Киберстаб Клиент\n")
	b.WriteString("Comment=Запуск клиента Киберстаб\n")
	b.WriteString(fmt.Sprintf("Exec=%q\n", execPath))
	b.WriteString(fmt.Sprintf("Path=%q\n", workDir))
	if icon != "" {
		b.WriteString(fmt.Sprintf("Icon=%s\n", icon))
	}
	b.WriteString("Terminal=false\n")
	b.WriteString("Categories=Application;\n")
	b.WriteString("StartupNotify=true\n")
	return b.String()
}

func writeDesktopEntry(path, content string, mode os.FileMode, ownerUser string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(path), ".desktop") {
		_ = os.Chmod(path, 0755)
	}
	chownDesktopEntry(path, ownerUser)
	markDesktopEntryTrusted(path, ownerUser)
	return nil
}

func chownDesktopEntry(path, ownerUser string) {
	if os.Geteuid() != 0 {
		return
	}
	ownerUser = strings.TrimSpace(ownerUser)
	if ownerUser == "" {
		ownerUser = strings.TrimSpace(os.Getenv("SUDO_USER"))
	}
	if ownerUser == "" || ownerUser == "root" {
		return
	}
	u, err := user.Lookup(ownerUser)
	if err != nil {
		return
	}
	uid, err1 := strconv.Atoi(u.Uid)
	gid, err2 := strconv.Atoi(u.Gid)
	if err1 != nil || err2 != nil {
		return
	}
	if err := os.Chown(path, uid, gid); err != nil {
		log.Printf("[INSTALL] WARN: chown %s to %s: %v", path, ownerUser, err)
	}
}

func markDesktopEntryTrusted(path, ownerUser string) {
	// Fly DE / Astra: untrusted .desktop files on the desktop are hidden until marked trusted.
	gio, err := exec.LookPath("gio")
	if err != nil {
		return
	}
	ownerUser = strings.TrimSpace(ownerUser)
	if ownerUser == "" {
		ownerUser = strings.TrimSpace(os.Getenv("SUDO_USER"))
	}
	var cmd *exec.Cmd
	if ownerUser != "" && ownerUser != "root" && os.Geteuid() == 0 {
		cmd = exec.Command("runuser", "-u", ownerUser, "--", gio, "set", path, "metadata::trusted", "true")
	} else {
		cmd = exec.Command(gio, "set", path, "metadata::trusted", "true")
	}
	_ = cmd.Run()
}
