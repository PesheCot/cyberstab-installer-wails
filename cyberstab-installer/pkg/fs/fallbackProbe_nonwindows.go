//go:build !windows

package fs

func fallbackProbeDistros(targets []string) []string {
	return nil
}

