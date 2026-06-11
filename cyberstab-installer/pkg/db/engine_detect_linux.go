//go:build linux

package db

import (
	"path/filepath"
	"strings"
)

func detectEngineKindByPath(binDir string) EngineKind {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(binDir)))
	switch {
	case strings.Contains(p, "jatoba"):
		return EngineJatoba
	case strings.Contains(p, "pgpro"),
		strings.Contains(p, "postgrespro"),
		strings.Contains(p, "/pgsql-"),
		strings.Contains(p, "/usr/pgsql/"):
		return EnginePostgresPro
	default:
		return EnginePostgreSQL
	}
}
