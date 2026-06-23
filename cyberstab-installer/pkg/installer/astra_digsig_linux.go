//go:build linux

package installer

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const astraDigsigKeyDir = "/etc/digsig/keys"

// installAstraDigsigKey copies the Cyberstab ZPS (*.key) into /etc/digsig/keys on Astra SE.
func installAstraDigsigKey(installDir string) error {
	osInfo, err := DetectLinuxOS()
	if err != nil || osInfo.Type != "ASTRA" || osInfo.Edition != "se" {
		return nil
	}

	keyDir := filepath.Join(installDirOrDefault(installDir), "CyberstabServerLinux", "server", "data", "certificate")
	keyFile, err := findFirstFile(keyDir, ".key")
	if err != nil {
		return err
	}
	if keyFile == "" {
		log.Printf("[DIGSIG] no *.key in %s, skip", keyDir)
		return nil
	}

	log.Printf("[DIGSIG] installing ZPS key: %s", keyFile)
	if err := os.MkdirAll(astraDigsigKeyDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать %s: %w", astraDigsigKeyDir, err)
	}

	dst := filepath.Join(astraDigsigKeyDir, filepath.Base(keyFile))
	if err := copyFileAtomic(keyFile, dst, 0644); err != nil {
		return fmt.Errorf("не удалось скопировать ключ ЗПС: %w", err)
	}
	log.Printf("[DIGSIG] key copied to %s", dst)

	if _, err := exec.LookPath("update-initramfs"); err != nil {
		log.Printf("[DIGSIG] WARN: update-initramfs not found, key installed but initramfs not updated")
		return nil
	}
	log.Printf("[DIGSIG] updating initramfs (may take a while)…")
	if err := runLinuxCmdLogged("update-initramfs", "-u", "-k", "all"); err != nil {
		log.Printf("[DIGSIG] WARN: update-initramfs: %v", err)
		return fmt.Errorf("ключ установлен, но update-initramfs завершился с ошибкой: %w", err)
	}
	log.Printf("[DIGSIG] initramfs updated; reboot may be required for ZPS")
	return nil
}

func findFirstFile(root, ext string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", nil
	}
	st, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !st.IsDir() {
		return "", nil
	}

	ext = strings.ToLower(ext)
	var hit string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) == ext {
			hit = path
			return filepath.SkipAll
		}
		return nil
	})
	return hit, err
}
