package system

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultNetworkPort    = 2017
	DefaultManagementPort = 2020
)

type ServerPorts struct {
	NetworkPort    int
	ManagementPort int
	PropertiesPath string
}

func installDirOrDefault(installDir string) string {
	if strings.TrimSpace(installDir) != "" {
		return filepath.Clean(installDir)
	}
	if runtime.GOOS == "windows" {
		return DefaultInstallDir
	}
	return DefaultInstallDir
}

func FindServerPropertiesPath(installDir string) (string, error) {
	installDir = installDirOrDefault(installDir)
	candidates := []string{
		filepath.Join(installDir, "server.properties"),
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(installDir, "CyberstabServerWindows", "server", "server.properties"),
			filepath.Join(installDir, "CyberstabServerWindows", "config", "server.properties"),
		)
	} else {
		candidates = append(candidates,
			filepath.Join(installDir, "CyberstabServerLinux", "server", "server.properties"),
			filepath.Join(installDir, "CyberstabServerLinux", "config", "server.properties"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}

	var hit string
	_ = filepath.WalkDir(installDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(installDir, path)
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) > 5 {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), "server.properties") {
			hit = path
			return filepath.SkipAll
		}
		return nil
	})
	if hit == "" {
		return "", fmt.Errorf("server.properties не найден в %s", installDir)
	}
	return hit, nil
}

func LoadServerPorts(installDir string) ServerPorts {
	ports := ServerPorts{
		NetworkPort:    DefaultNetworkPort,
		ManagementPort: DefaultManagementPort,
	}
	path, err := FindServerPropertiesPath(installDir)
	if err != nil {
		return ports
	}
	ports.PropertiesPath = path
	b, err := os.ReadFile(path)
	if err != nil {
		return ports
	}
	if v, ok := parsePropertiesInt(b, "network.portnumber"); ok {
		ports.NetworkPort = v
	}
	if v, ok := parsePropertiesInt(b, "management.portnumber"); ok {
		ports.ManagementPort = v
	}
	return ports
}

func parsePropertiesInt(data []byte, key string) (int, bool) {
	want := strings.ToLower(strings.TrimSpace(key))
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parts[0])) != want {
			continue
		}
		val := strings.TrimSpace(parts[1])
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 || n > 65535 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
