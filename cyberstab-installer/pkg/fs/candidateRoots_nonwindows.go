//go:build !windows

package fs

import (
	"os"
	"path/filepath"
)

// candidateRoots returns mount points where removable USB media typically appear.
// We intentionally do NOT scan "/" — that would match already-installed copies
// under /opt/cyberstab, Desktop, etc. and bypass the USB check in the wizard.
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
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p1 := filepath.Join(base, e.Name())
			add(p1)
			// /run/media/<user>/<label> or /media/<label>
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
	return roots
}
