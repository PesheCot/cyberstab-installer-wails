//go:build windows

package db

import (
	"strings"
)

func detectEngineKindByPath(binDir string) EngineKind {
	p := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(binDir), "/", `\`))
	if strings.Contains(p, `\gis\jatoba\`) || strings.Contains(p, `\jatoba\`) {
		return EngineJatoba
	}
	if strings.Contains(p, `\pgpro\`) || strings.Contains(p, `\postgrespro\`) {
		return EnginePostgresPro
	}
	return EnginePostgreSQL
}
