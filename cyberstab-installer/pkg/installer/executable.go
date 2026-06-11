package installer

import (
	"strings"
)

func isELFBytes(b []byte) bool {
	return len(b) >= 4 && b[0] == 0x7f && b[1] == 'E' && b[2] == 'L' && b[3] == 'F'
}

func looksLikeInstallBinary(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	if strings.HasSuffix(lower, ".sh") {
		return true
	}
	if strings.Contains(lower, ".") {
		return false
	}
	switch lower {
	case "dbupdater", "serverconsole":
		return true
	}
	return strings.Contains(lower, "cyberstab")
}
