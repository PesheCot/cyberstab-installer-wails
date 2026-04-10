//go:build windows

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fallbackProbeDistros checks all drive roots except C:\ and returns the first-level parent
// for any root that contains matching Cyberstab folders.
// This is intentionally shallow and fast (no recursion).
func fallbackProbeDistros(targets []string) []string {
	mask, _, _ := procGetLogicalDrv.Call()
	if mask == 0 {
		return nil
	}

	want := make([]string, 0, len(targets))
	for _, t := range targets {
		if strings.TrimSpace(t) == "" {
			continue
		}
		want = append(want, strings.ToLower(t))
	}
	if len(want) == 0 {
		return nil
	}

	var hits []string
	for i := 0; i < 26; i++ {
		if (mask & (1 << uint(i))) == 0 {
			continue
		}
		letter := byte('A' + i)
		if letter == 'C' {
			continue
		}
		root := fmt.Sprintf("%c:\\", letter)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			nameLower := strings.ToLower(e.Name())
			for _, w := range want {
				if nameLower == w || strings.HasPrefix(nameLower, w) {
					hits = append(hits, filepath.Clean(root))
					goto nextDrive
				}
			}
		}
	nextDrive:
	}
	return uniqueStrings(hits)
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

