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
)

var hbaPasswordMethods = map[string]struct{}{
	"md5":           {},
	"password":      {},
	"scram-sha-256": {},
	"scram":         {},
}

var hbaPathRe = regexp.MustCompile(`/etc/postgresql/([^/]+)/([^/]+)/pg_hba\.conf`)

func setUserPasswordPlatform(username, newPassword string) error {
	sql := alterUserPasswordSQL(username, newPassword)
	if _, err := runPSQLAsLocalSuperuser("postgres", sql); err == nil {
		return nil
	} else if !isPostgresAuthError(err) || os.Geteuid() != 0 {
		return fmt.Errorf("не удалось сменить пароль для %s: %w", username, err)
	}

	log.Printf("[DB] peer auth failed, temporarily switching pg_hba.conf local rules to peer")
	if err := withTemporaryLocalPeerAuth(func() error {
		_, runErr := runPSQLAsLocalSuperuser("postgres", sql)
		return runErr
	}); err != nil {
		return fmt.Errorf("не удалось сменить пароль для %s: %w", username, err)
	}
	return nil
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
		strings.Contains(lower, "peer-доступ")
}

func findPgHbaConf() (string, error) {
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
	return "", fmt.Errorf("pg_hba.conf не найден")
}

func patchHbaToLocalPeer(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		newLine, lineChanged := patchHbaLineToPeer(line)
		if lineChanged {
			changed = true
			lines[i] = newLine
		}
	}
	return strings.Join(lines, "\n"), changed
}

func patchHbaLineToPeer(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return line, false
	}
	fields := strings.Fields(line)
	if len(fields) < 4 || !strings.EqualFold(fields[0], "local") {
		return line, false
	}
	method := strings.ToLower(fields[len(fields)-1])
	if method == "peer" || method == "trust" {
		return line, false
	}
	if _, ok := hbaPasswordMethods[method]; !ok {
		return line, false
	}
	fields[len(fields)-1] = "peer"
	return strings.Join(fields, "\t"), true
}

func withTemporaryLocalPeerAuth(fn func() error) error {
	hbaPath, err := findPgHbaConf()
	if err != nil {
		return err
	}
	original, err := os.ReadFile(hbaPath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать %s: %w", hbaPath, err)
	}
	patched, changed := patchHbaToLocalPeer(string(original))
	if !changed {
		return fmt.Errorf("в %s нет local-правил с md5/scram для временной смены на peer", hbaPath)
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
		reloadPostgresFromHbaPath(hbaPath)
	}()

	if err := os.WriteFile(hbaPath, []byte(patched), 0600); err != nil {
		return fmt.Errorf("не удалось записать %s: %w", hbaPath, err)
	}
	log.Printf("[DB] pg_hba.conf temporarily patched (local -> peer): %s", hbaPath)
	reloadPostgresFromHbaPath(hbaPath)

	if err := fn(); err != nil {
		return err
	}
	return nil
}

func reloadPostgresFromHbaPath(hbaPath string) {
	if m := hbaPathRe.FindStringSubmatch(hbaPath); len(m) == 3 {
		if pgCtl, err := exec.LookPath("pg_ctlcluster"); err == nil {
			cmd := exec.Command(pgCtl, m[1], m[2], "reload")
			if err := cmd.Run(); err == nil {
				log.Printf("[DB] reloaded cluster %s %s", m[1], m[2])
				return
			}
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
