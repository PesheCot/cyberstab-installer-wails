//go:build windows

package system

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

func removeCyberstabUninstallRegistry(installDir string) error {
	installDir = strings.ToLower(strings.TrimSpace(installDir))
	if installDir != "" {
		installDir = strings.TrimRight(installDir, `\`)
	}

	roots := []struct {
		k    registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
	}

	for _, r := range roots {
		// Try 64-bit view when possible.
		_ = cleanUninstallRoot(r.k, r.path, installDir, registry.WOW64_64KEY)
		_ = cleanUninstallRoot(r.k, r.path, installDir, registry.WOW64_32KEY)
		_ = cleanUninstallRoot(r.k, r.path, installDir, 0)
	}
	return nil
}

func cleanUninstallRoot(root registry.Key, path string, installDir string, view uint32) error {
	k, err := registry.OpenKey(root, path, registry.READ|registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE|view)
	if err != nil {
		return nil
	}
	defer k.Close()

	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	for _, name := range names {
		sub, err := registry.OpenKey(root, path+`\`+name, registry.READ|registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS|view)
		if err != nil {
			continue
		}
		displayName, _, _ := sub.GetStringValue("DisplayName")
		installLoc, _, _ := sub.GetStringValue("InstallLocation")
		uninstallStr, _, _ := sub.GetStringValue("UninstallString")
		sub.Close()

		match := false
		if strings.Contains(strings.ToLower(displayName), "cyberstab") {
			match = true
		}
		if !match && strings.TrimSpace(installLoc) != "" && installDir != "" {
			loc := strings.ToLower(strings.TrimRight(strings.TrimSpace(installLoc), `\`))
			if loc == installDir {
				match = true
			}
		}
		if !match && strings.Contains(strings.ToLower(uninstallStr), "cyberstab") {
			match = true
		}

		if match {
			_ = deleteRegistryTree(root, path+`\`+name, view)
		}
	}
	return nil
}

func deleteRegistryTree(root registry.Key, path string, view uint32) error {
	k, err := registry.OpenKey(root, path, registry.READ|registry.ENUMERATE_SUB_KEYS|view)
	if err == nil {
		subKeys, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, sk := range subKeys {
			_ = deleteRegistryTree(root, path+`\`+sk, view)
		}
	}
	// Finally delete this key.
	_ = registry.DeleteKey(root, path) // will fail if not empty; but we've tried to clear children
	return nil
}

