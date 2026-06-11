//go:build !linux && !windows

package db

func detectEngineKindByPath(binDir string) EngineKind {
	return EnginePostgreSQL
}
