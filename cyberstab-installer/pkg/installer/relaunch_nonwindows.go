//go:build !windows

package installer

func tryRelaunchAsAdmin(args []string) bool {
	return false
}

