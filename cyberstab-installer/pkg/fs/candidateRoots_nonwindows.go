//go:build !windows

package fs

func candidateRoots() []string {
	return []string{"/mnt", "/media", "/run/media", "/"}
}

