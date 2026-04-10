//go:build !windows

package installer

import "os"

func needSudo() bool {
	return os.Geteuid() != 0
}

