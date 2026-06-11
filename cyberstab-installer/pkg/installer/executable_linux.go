//go:build linux

package installer

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

func isELF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	return isELFBytes(hdr[:])
}

func ensureExecutable(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return nil
	}
	if st.Mode()&0111 != 0 {
		return nil
	}
	if !isELF(path) && !looksLikeInstallBinary(filepath.Base(path)) {
		return nil
	}
	if err := os.Chmod(path, st.Mode()|0755); err != nil {
		return err
	}
	log.Printf("[INSTALL] chmod +x %s", path)
	return nil
}

func ensureInstallExecutablesLinux(installDir string) error {
	for _, root := range []string{
		filepath.Join(installDir, "CyberstabServerLinux"),
		filepath.Join(installDir, "CyberstabClientLinux64"),
		filepath.Join(installDir, "CyberstabClientLinux32"),
	} {
		st, err := os.Stat(root)
		if err != nil || !st.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if isELF(path) || looksLikeInstallBinary(d.Name()) {
				_ = ensureExecutable(path)
			}
			return nil
		})
	}
	return nil
}
