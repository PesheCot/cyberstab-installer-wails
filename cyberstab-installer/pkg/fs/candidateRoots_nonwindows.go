//go:build !windows

package fs

import (
	"os"
	"path/filepath"
	"strings"
)

// candidateRoots returns mount points where removable USB media typically appear.
// Covers udisks2/udisks paths on Ubuntu, Debian, Astra, ALT, RED OS, Основа, etc.
func candidateRoots() []string {
	var roots []string
	seen := map[string]struct{}{}
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		seen[p] = struct{}{}
		roots = append(roots, p)
	}

	for _, base := range []string{"/run/media", "/media", "/mnt"} {
		add(base)
		walkMountTree(base, add, 2)
	}

	// systemd user sessions: /run/user/<uid>/media/<label> (Astra Fly, some GNOME setups)
	runUser := "/run/user"
	if entries, err := os.ReadDir(runUser); err == nil {
		for _, u := range entries {
			if !u.IsDir() {
				continue
			}
			mediaBase := filepath.Join(runUser, u.Name(), "media")
			add(mediaBase)
			walkMountTree(mediaBase, add, 1)
		}
	}

	// Fallback: parse /proc/mounts for vfat/exfat/ntfs and removable ext* under /media|/mnt
	for _, mp := range mountPointsFromProc() {
		add(mp)
	}

	return roots
}

func walkMountTree(base string, add func(string), maxDepth int) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p1 := filepath.Join(base, e.Name())
		add(p1)
		if maxDepth > 1 {
			sub, err := os.ReadDir(p1)
			if err != nil {
				continue
			}
			for _, s := range sub {
				if s.IsDir() {
					add(filepath.Join(p1, s.Name()))
				}
			}
		}
	}
}

func mountPointsFromProc() []string {
	b, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	removableFS := map[string]bool{
		"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true,
		"fuseblk": true, "udf": true, "iso9660": true,
	}
	var out []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mp := unescapeProcMount(fields[1])
		fstype := fields[2]
		if isSystemMountPoint(mp) {
			continue
		}
		if removableFS[fstype] {
			if _, ok := seen[mp]; !ok {
				seen[mp] = struct{}{}
				out = append(out, mp)
			}
			continue
		}
		if (fstype == "ext4" || fstype == "ext3" || fstype == "btrfs") && isRemovableMountParent(mp) {
			if _, ok := seen[mp]; !ok {
				seen[mp] = struct{}{}
				out = append(out, mp)
			}
		}
	}
	return out
}

func unescapeProcMount(s string) string {
	s = strings.ReplaceAll(s, "\\040", " ")
	s = strings.ReplaceAll(s, "\\011", "\t")
	return s
}

func isRemovableMountParent(mp string) bool {
	for _, prefix := range []string{"/media/", "/mnt/", "/run/media/", "/run/user/"} {
		if strings.HasPrefix(mp, prefix) {
			return true
		}
	}
	return false
}

func isSystemMountPoint(mp string) bool {
	if mp == "/" {
		return true
	}
	for _, prefix := range []string{
		"/proc", "/sys", "/dev", "/boot", "/snap", "/var/lib",
		"/opt/cyberstab", "/home", "/usr", "/tmp",
	} {
		if mp == prefix || strings.HasPrefix(mp, prefix+"/") {
			return true
		}
	}
	return false
}
