package fs

import (
	"os"
	"path/filepath"
	"strings"
)

// Finder locates Cyberstab distro folders on connected drives.
// It is intentionally conservative to avoid long scans.
type Finder struct {
	// ServerDirNames are directory names that must exist under a distro root.
	// Example: CyberstabServerWindows, CyberstabServerLinux, etc.
	ServerDirNames []string
}

func NewFinder() *Finder {
	// Defaults: keep a superset; callers can override.
	return &Finder{
		ServerDirNames: []string{
			"CyberstabServerWindows",
			"CyberstabServerLinux",
			"CyberstabClientWindows32",
			"CyberstabClientWindows64",
			"CyberstabClientLinux32",
			"CyberstabClientLinux64",
		},
	}
}

// FindDistros returns parent directories that contain at least one of ServerDirNames.
// On Windows it probes drive roots (A:\..Z:\) and a few common removable mount points.
func (f *Finder) FindDistros() []string {
	var roots []string
	candidates := candidateRoots()
	seen := map[string]struct{}{}

	for _, root := range candidates {
		root = filepath.Clean(root)
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}

		found := findDistroUnder(root, f.ServerDirNames)
		for _, p := range found {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			roots = append(roots, p)
		}
	}

	// Windows fallback: if we couldn't determine USB roots reliably (PowerShell blocked, drive types lie),
	// do a very fast probe of all drive letters except C:\, checking only top-level directories.
	if len(roots) == 0 {
		roots = append(roots, fallbackProbeDistros(f.ServerDirNames)...)
	}
	return roots
}

func findDistroUnder(root string, dirNames []string) []string {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return nil
	}

	// Scan only a limited depth to keep it fast.
	const maxDepth = 4
	var hits []string

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel != "." && strings.Count(rel, string(os.PathSeparator)) > maxDepth {
			return filepath.SkipDir
		}

		name := d.Name()
		nameLower := strings.ToLower(name)
		for _, n := range dirNames {
			nLower := strings.ToLower(n)
			// Accept exact match and prefix match (e.g. CyberstabServerWindows49-1...).
			if nameLower == nLower || strings.HasPrefix(nameLower, nLower) {
				parent := filepath.Dir(path)
				hits = append(hits, parent)
				return filepath.SkipDir
			}
		}
		return nil
	})

	return hits
}

