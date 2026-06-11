//go:build linux

package db

import (
	"os"
	"path/filepath"
	"sort"
)

func discoverAdditionalEngines(addBin func(string)) {
	seen := map[string]bool{}
	for _, bin := range findAllPostgresProBins() {
		key := filepath.Clean(bin)
		if seen[key] {
			continue
		}
		seen[key] = true
		addBin(bin)
	}
}

func findAllPostgresProBins() []string {
	var candidates []string
	for _, pattern := range []string{
		"/opt/pgpro/*/bin",
		"/opt/pgpro/std-*/bin",
		"/opt/pgpro/pgpro-*/bin",
		"/opt/postgrespro/*/bin",
		"/usr/lib/pgpro/*/bin",
		"/usr/pgsql-*/bin",
		"/usr/pgsql/*/bin",
	} {
		found, _ := filepath.Glob(pattern)
		candidates = append(candidates, found...)
	}
	for _, base := range []string{"/opt/pgpro", "/usr/lib/pgpro", "/opt/postgrespro"} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p1 := filepath.Join(base, e.Name(), "bin")
			if hasPsql(p1) {
				candidates = append(candidates, p1)
			}
			sub, err := os.ReadDir(filepath.Join(base, e.Name()))
			if err != nil {
				continue
			}
			for _, s := range sub {
				if !s.IsDir() {
					continue
				}
				p2 := filepath.Join(base, e.Name(), s.Name(), "bin")
				if hasPsql(p2) {
					candidates = append(candidates, p2)
				}
			}
		}
	}
	sort.Strings(candidates)
	return uniqueStrings(candidates)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = filepath.Clean(s)
		if s == "" || seen[s] {
			continue
		}
		if !hasPsql(s) {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
