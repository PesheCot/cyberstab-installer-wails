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

var (
	hbaPathDebianRe = regexp.MustCompile(`/etc/postgresql/([^/]+)/([^/]+)/pg_hba\.conf`)
	hbaPathPgProRe  = regexp.MustCompile(`/var/lib/pgpro/([^/]+)(?:/([^/]+))?/data/pg_hba\.conf`)
)

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
	engine := GetActiveEngine()
	if p := findPgHbaNearEngine(engine); p != "" {
		return p, nil
	}
	if p, err := findPgHbaViaClusters(); err == nil && p != "" {
		return p, nil
	}
	if p := findPgHbaViaSystemd(engine); p != "" {
		return p, nil
	}
	candidates := collectHbaCandidates()
	if p := pickBestHbaCandidate(candidates, engine); p != "" {
		return p, nil
	}
	label := engine.DisplayName
	if label == "" {
		label = "СУБД"
	}
	return "", fmt.Errorf("pg_hba.conf не найден для %s (bin: %s)", label, engine.BinDir)
}

func findPgHbaNearEngine(engine EngineInfo) string {
	if strings.TrimSpace(engine.BinDir) == "" {
		return ""
	}
	pgConfig := filepath.Join(engine.BinDir, "pg_config")
	if st, err := os.Stat(pgConfig); err == nil && !st.IsDir() {
		for _, flag := range []string{"--sysconfdir", "--sharedir"} {
			out, err := exec.Command(pgConfig, flag).Output()
			if err != nil {
				continue
			}
			dir := strings.TrimSpace(string(out))
			for _, cand := range []string{
				filepath.Join(dir, "pg_hba.conf"),
				filepath.Join(dir, "..", "data", "pg_hba.conf"),
			} {
				if fileExists(cand) {
					return cand
				}
			}
		}
	}

	root := filepath.Dir(engine.BinDir)
	for _, rel := range []string{
		"data/pg_hba.conf",
		"../data/pg_hba.conf",
	} {
		if fileExists(filepath.Join(root, rel)) {
			return filepath.Join(root, rel)
		}
	}

	if engine.Version != "" {
		for _, pattern := range []string{
			filepath.Join("/var/lib/pgpro", "*", "data", "pg_hba.conf"),
			filepath.Join("/var/lib/pgpro", engine.Version, "*", "data", "pg_hba.conf"),
			filepath.Join("/var/lib/pgpro", "*", engine.Version, "data", "pg_hba.conf"),
			filepath.Join("/opt/pgpro", "*", "data", "pg_hba.conf"),
			filepath.Join("/opt/pgpro", "std-"+engine.Version, "data", "pg_hba.conf"),
			filepath.Join("/opt/pgpro", "pgpro-"+engine.Version, "data", "pg_hba.conf"),
		} {
			for _, cand := range globFiles(pattern) {
				if fileExists(cand) {
					return cand
				}
			}
		}
	}
	return ""
}

func findPgHbaViaSystemd(engine EngineInfo) string {
	units := systemdUnitsForEngine(engine)
	for _, unit := range units {
		if pgdata := pgDataFromSystemdUnit(unit); pgdata != "" {
			hba := filepath.Join(pgdata, "pg_hba.conf")
			if fileExists(hba) {
				return hba
			}
		}
	}
	return ""
}

func systemdUnitsForEngine(engine EngineInfo) []string {
	var units []string
	if engine.Version != "" {
		units = append(units,
			fmt.Sprintf("pgpro-%s", engine.Version),
			fmt.Sprintf("postgrespro-%s", engine.Version),
			fmt.Sprintf("postgresql@%s-main", engine.Version),
		)
	}
	switch engine.Kind {
	case EnginePostgresPro:
		units = append(units, "pgpro", "postgrespro")
	case EnginePostgreSQL:
		units = append(units, "postgresql")
	}
	units = append(units,
		"pgpro-18", "pgpro-17", "pgpro-16", "pgpro-15", "pgpro-14",
		"postgresql@18-main", "postgresql@17-main", "postgresql@16-main",
	)
	return uniqueNonEmpty(units)
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func pgDataFromSystemdUnit(unit string) string {
	out, err := exec.Command("systemctl", "show", unit, "-p", "Environment", "--value").Output()
	if err == nil {
		if pgdata := parsePGDATA(string(out)); pgdata != "" {
			return pgdata
		}
	}
	out, err = exec.Command("systemctl", "cat", unit).Output()
	if err != nil {
		return ""
	}
	return parsePGDATA(string(out))
}

var pgDataEnvRe = regexp.MustCompile(`(?m)(?:^|\s)PGDATA=(?:\"([^\"]+)\"|([^\s\"]+))`)

func parsePGDATA(text string) string {
	m := pgDataEnvRe.FindStringSubmatch(text)
	if len(m) < 3 {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

func collectHbaCandidates() []string {
	patterns := []string{
		"/etc/postgresql/*/main/pg_hba.conf",
		"/etc/postgresql/*/pg_hba.conf",
		"/var/lib/postgresql/*/main/pg_hba.conf",
		"/var/lib/postgresql/*/data/pg_hba.conf",
		"/var/lib/pgpro/*/data/pg_hba.conf",
		"/var/lib/pgpro/*/*/data/pg_hba.conf",
		"/opt/pgpro/*/data/pg_hba.conf",
		"/opt/pgpro/*/*/data/pg_hba.conf",
		"/opt/pgpro/std-*/data/pg_hba.conf",
		"/opt/pgpro/pgpro-*/data/pg_hba.conf",
		"/var/lib/pgsql/data/pg_hba.conf",
		"/var/lib/pgsql/*/data/pg_hba.conf",
	}
	var out []string
	for _, pattern := range patterns {
		out = append(out, globFiles(pattern)...)
	}
	return uniqueStrings(out)
}

func pickBestHbaCandidate(candidates []string, engine EngineInfo) string {
	if len(candidates) == 0 {
		return ""
	}
	if engine.BinDir != "" {
		binKey := strings.ToLower(filepath.ToSlash(filepath.Dir(engine.BinDir)))
		for _, c := range candidates {
			cl := strings.ToLower(filepath.ToSlash(c))
			if strings.Contains(cl, binKey) {
				return c
			}
		}
	}
	if engine.Version != "" {
		for _, c := range candidates {
			if strings.Contains(c, engine.Version) {
				return c
			}
		}
	}
	sort.Strings(candidates)
	return candidates[len(candidates)-1]
}

func globFiles(pattern string) []string {
	found, _ := filepath.Glob(pattern)
	return found
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
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
		if fileExists(hba) {
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

	dataDir := dataDirFromHbaPath(hbaPath)
	if dataDir == "" {
		return
	}
	info, err := CheckPostgres()
	if err == nil && info != nil && info.Installed {
		pgCtl := filepath.Join(info.BinDir, "pg_ctl")
		cmd := exec.Command("runuser", "-u", "postgres", "--", pgCtl, "reload", "-D", dataDir)
		if err := cmd.Run(); err == nil {
			log.Printf("[DB] pg_ctl reload -D %s", dataDir)
			time.Sleep(400 * time.Millisecond)
		}
	}
}

func dataDirFromHbaPath(hbaPath string) string {
	if strings.HasSuffix(hbaPath, "/pg_hba.conf") {
		dir := filepath.Dir(hbaPath)
		if strings.HasSuffix(dir, "/data") || strings.HasSuffix(dir, `\data`) {
			return dir
		}
	}
	if m := hbaPathDebianRe.FindStringSubmatch(hbaPath); len(m) == 3 {
		return filepath.Join("/var/lib/postgresql", m[1], m[2])
	}
	if m := hbaPathPgProRe.FindStringSubmatch(hbaPath); len(m) >= 2 {
		if len(m) >= 3 && m[2] != "" {
			return filepath.Join("/var/lib/pgpro", m[1], m[2], "data")
		}
		return filepath.Join("/var/lib/pgpro", m[1], "data")
	}
	return ""
}

func reloadPostgresFromHbaPath(hbaPath string) {
	engine := GetActiveEngine()
	if m := hbaPathDebianRe.FindStringSubmatch(hbaPath); len(m) == 3 {
		if pgCtl, err := exec.LookPath("pg_ctlcluster"); err == nil {
			cmd := exec.Command(pgCtl, m[1], m[2], "reload")
			if err := cmd.Run(); err == nil {
				log.Printf("[DB] pg_ctlcluster %s %s reload", m[1], m[2])
				return
			}
		}
		unit := fmt.Sprintf("postgresql@%s-%s", m[1], m[2])
		if reloadSystemdUnit(unit) {
			return
		}
	}
	if engine.Version != "" {
		for _, unit := range []string{
			fmt.Sprintf("pgpro-%s", engine.Version),
			fmt.Sprintf("postgrespro-%s", engine.Version),
			"pgpro",
			"postgrespro",
		} {
			if reloadSystemdUnit(unit) {
				return
			}
		}
	}
	reloadPostgresServiceBestEffort()
}

func reloadSystemdUnit(unit string) bool {
	cmd := exec.Command("systemctl", "reload", unit)
	if err := cmd.Run(); err == nil {
		log.Printf("[DB] systemctl reload %s", unit)
		return true
	}
	return false
}

func reloadPostgresServiceBestEffort() {
	for _, unit := range []string{
		"postgresql",
		"pgpro-18", "pgpro-17", "pgpro-16", "pgpro-15", "pgpro-14",
		"postgrespro",
		"postgresql@18-main", "postgresql@17-main", "postgresql@16-main",
		"postgresql@15-main", "postgresql@14-main",
	} {
		if reloadSystemdUnit(unit) {
			return
		}
	}
}
