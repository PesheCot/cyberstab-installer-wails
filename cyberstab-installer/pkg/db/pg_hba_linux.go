//go:build linux

package db

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var hbaPasswordMethods = map[string]struct{}{
	"md5":           {},
	"password":      {},
	"scram-sha-256": {},
	"scram":         {},
	"ident":         {},
}

var hbaPathRe = regexp.MustCompile(`/etc/postgresql/([^/]+)/([^/]+)/pg_hba\.conf`)

func setUserPasswordPlatform(username, newPassword string) error {
	sql := alterUserPasswordSQL(username, newPassword)
	if os.Geteuid() == 0 {
		if err := setUserPasswordWithHbaPatch(sql); err != nil {
			return fmt.Errorf("не удалось сменить пароль для %s: %w", username, err)
		}
		return nil
	}
	if _, err := runPSQLAsLocalSuperuser("postgres", sql); err != nil {
		return fmt.Errorf("не удалось сменить пароль для %s: %w", username, err)
	}
	return nil
}

func setUserPasswordWithHbaPatch(sql string) error {
	if _, err := runPSQLSuperuserMulti("postgres", sql); err == nil {
		return nil
	} else if !isPostgresAuthError(err) {
		return err
	}

	log.Printf("[DB] auth failed, temporarily relaxing pg_hba.conf (local→peer, localhost→trust)")
	return withTemporaryHbaAuthRelax(func() error {
		_, runErr := runPSQLSuperuserMulti("postgres", sql)
		return runErr
	})
}

func isPostgresAuthError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "password authentication") ||
		strings.Contains(lower, "проверку подлинности") ||
		strings.Contains(lower, "проверка подлинности") ||
		strings.Contains(lower, "peer-доступ") ||
		strings.Contains(lower, "не прошёл проверку") ||
		strings.Contains(lower, "не прошел проверку")
}

func findPgHbaConf() (string, error) {
	if p, err := findPgHbaViaClusters(); err == nil && p != "" {
		return p, nil
	}
	var matches []string
	for _, pattern := range []string{
		"/etc/postgresql/*/main/pg_hba.conf",
		"/etc/postgresql/*/pg_hba.conf",
	} {
		found, _ := filepath.Glob(pattern)
		matches = append(matches, found...)
	}
	sort.Strings(matches)
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	for _, p := range []string{
		"/var/lib/postgresql/data/pg_hba.conf",
		"/var/lib/pgsql/data/pg_hba.conf",
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("pg_hba.conf не найден (проверьте /etc/postgresql/*/main/pg_hba.conf)")
}

func findPgHbaViaClusters() (string, error) {
	pgLs, err := exec.LookPath("pg_lsclusters")
	if err != nil {
		return "", err
	}
	out, err := exec.Command(pgLs, "--no-header").Output()
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		hba := filepath.Join("/etc/postgresql", fields[0], fields[1], "pg_hba.conf")
		if st, statErr := os.Stat(hba); statErr == nil && !st.IsDir() {
			candidates = append(candidates, hba)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("pg_lsclusters: hba not found")
	}
	sort.Strings(candidates)
	return candidates[len(candidates)-1], nil
}

func patchHbaForPasswordReset(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		newLine, lineChanged := patchHbaLineForPasswordReset(line)
		if lineChanged {
			changed = true
			lines[i] = newLine
		}
	}
	return strings.Join(lines, "\n"), changed
}

func patchHbaLineForPasswordReset(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return line, false
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return line, false
	}
	connType := strings.ToLower(fields[0])
	method := strings.ToLower(fields[len(fields)-1])
	if method == "peer" || method == "trust" {
		return line, false
	}
	if _, ok := hbaPasswordMethods[method]; !ok {
		return line, false
	}

	switch connType {
	case "local":
		fields[len(fields)-1] = "peer"
		return strings.Join(fields, "\t"), true
	case "host", "hostssl", "hostnossl":
		if len(fields) < 5 {
			return line, false
		}
		address := fields[len(fields)-2]
		if address == "127.0.0.1/32" || address == "::1/128" || strings.EqualFold(address, "localhost") {
			fields[len(fields)-1] = "trust"
			return strings.Join(fields, "\t"), true
		}
	}
	return line, false
}

func withTemporaryHbaAuthRelax(fn func() error) error {
	hbaPath, err := findPgHbaConf()
	if err != nil {
		return err
	}
	log.Printf("[DB] using pg_hba.conf: %s", hbaPath)

	original, err := os.ReadFile(hbaPath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать %s: %w", hbaPath, err)
	}
	patched, changed := patchHbaForPasswordReset(string(original))
	if !changed {
		return fmt.Errorf("в %s нет правил local/host с md5/scram для временной смены", hbaPath)
	}

	backupPath := hbaPath + ".cyberstab.bak"
	if err := os.WriteFile(backupPath, original, 0600); err != nil {
		return fmt.Errorf("не удалось создать резервную копию %s: %w", backupPath, err)
	}
	defer func() {
		if err := os.WriteFile(hbaPath, original, 0600); err != nil {
			log.Printf("[DB] WARNING: failed to restore %s: %v", hbaPath, err)
		} else {
			log.Printf("[DB] pg_hba.conf restored: %s", hbaPath)
		}
		_ = os.Remove(backupPath)
		reloadPostgresAggressive(hbaPath)
	}()

	if err := os.WriteFile(hbaPath, []byte(patched), 0600); err != nil {
		return fmt.Errorf("не удалось записать %s: %w", hbaPath, err)
	}
	log.Printf("[DB] pg_hba.conf patched (local→peer, localhost→trust): %s", hbaPath)
	reloadPostgresAggressive(hbaPath)

	if err := fn(); err != nil {
		return fmt.Errorf("после смены pg_hba: %w", err)
	}
	return nil
}

func reloadPostgresAggressive(hbaPath string) {
	reloadPostgresFromHbaPath(hbaPath)
	time.Sleep(800 * time.Millisecond)

	if m := hbaPathRe.FindStringSubmatch(hbaPath); len(m) == 3 {
		dataDir := filepath.Join("/var/lib/postgresql", m[1], m[2])
		info, err := CheckPostgres()
		if err == nil && info != nil && info.Installed {
			pgCtl := filepath.Join(info.BinDir, "pg_ctl")
			cmd := exec.Command("runuser", "-u", "postgres", "--", pgCtl, "reload", "-D", dataDir)
			if err := cmd.Run(); err == nil {
				log.Printf("[DB] pg_ctl reload -D %s", dataDir)
				time.Sleep(400 * time.Millisecond)
				return
			}
		}
	}
}

func reloadPostgresFromHbaPath(hbaPath string) {
	if m := hbaPathRe.FindStringSubmatch(hbaPath); len(m) == 3 {
		if pgCtl, err := exec.LookPath("pg_ctlcluster"); err == nil {
			cmd := exec.Command(pgCtl, m[1], m[2], "reload")
			if err := cmd.Run(); err == nil {
				log.Printf("[DB] pg_ctlcluster %s %s reload", m[1], m[2])
				return
			}
		}
		unit := fmt.Sprintf("postgresql@%s-%s", m[1], m[2])
		cmd := exec.Command("systemctl", "reload", unit)
		if err := cmd.Run(); err == nil {
			log.Printf("[DB] systemctl reload %s", unit)
			return
		}
	}
	reloadPostgresServiceBestEffort()
}

func reloadPostgresServiceBestEffort() {
	for _, unit := range []string{
		"postgresql",
		"postgresql@16-main",
		"postgresql@15-main",
		"postgresql@14-main",
		"postgresql@13-main",
		"postgresql@12-main",
		"postgresql@11-main",
	} {
		cmd := exec.Command("systemctl", "reload", unit)
		if err := cmd.Run(); err == nil {
			log.Printf("[DB] reloaded systemd unit %s", unit)
			return
		}
	}
}
