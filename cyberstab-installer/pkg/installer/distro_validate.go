package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DistroDirPrefixes returns top-level folder name prefixes required for the selected components.
func DistroDirPrefixes(wantServerOrDB, wantClients bool) []string {
	var prefixes []string
	if wantServerOrDB {
		if runtime.GOOS == "windows" {
			prefixes = append(prefixes, "CyberstabServerWindows")
		} else {
			prefixes = append(prefixes, "CyberstabServerLinux")
		}
	}
	if wantClients {
		if runtime.GOOS == "windows" {
			if is64BitWindows() {
				prefixes = append(prefixes, "CyberstabClientWindows64")
			} else {
				prefixes = append(prefixes, "CyberstabClientWindows32")
			}
		} else if is64BitLinux() {
			prefixes = append(prefixes, "CyberstabClientLinux64")
		} else {
			prefixes = append(prefixes, "CyberstabClientLinux32")
		}
	}
	return prefixes
}

// ValidateDistroRoot checks that root contains required Cyberstab distro folders.
// Returns a list of missing folder patterns (empty slice means OK).
func ValidateDistroRoot(root string, wantServerOrDB, wantClients bool) ([]string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return nil, fmt.Errorf("путь к дистрибутиву не указан")
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("папка не существует: %s", root)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("это не папка: %s", root)
	}

	prefixes := DistroDirPrefixes(wantServerOrDB, wantClients)
	if len(prefixes) == 0 {
		return nil, nil
	}
	found, err := selectTopLevelDirs(root, prefixes)
	if err != nil {
		return nil, err
	}

	foundPrefixes := map[string]bool{}
	for _, dir := range found {
		base := strings.ToLower(filepath.Base(dir))
		for _, p := range prefixes {
			pLower := strings.ToLower(p)
			if base == pLower || strings.HasPrefix(base, pLower) {
				foundPrefixes[p] = true
			}
		}
	}

	var missing []string
	for _, p := range prefixes {
		if !foundPrefixes[p] {
			missing = append(missing, p+"*")
		}
	}
	return missing, nil
}
