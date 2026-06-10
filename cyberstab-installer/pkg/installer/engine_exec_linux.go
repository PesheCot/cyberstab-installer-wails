//go:build linux

package installer

import (
	"os"
	"strings"
)

func isLikelyExecutable(path string, d os.DirEntry) bool {
	_ = path
	if strings.HasSuffix(strings.ToLower(d.Name()), ".exe") {
		return true
	}
	info, err := d.Info()
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}
