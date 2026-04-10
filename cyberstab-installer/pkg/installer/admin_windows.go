//go:build windows

package installer

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

func needSudo() bool {
	// Returns true when process is not elevated.
	// This is a best-effort check for "run as admin" requirement.
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		// If unsure, assume admin required.
		return true
	}
	defer token.Close()

	// TOKEN_ELEVATION structure
	type tokenElevation struct {
		TokenIsElevated uint32
	}
	var elevation tokenElevation
	var outLen uint32
	err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &outLen)
	if err != nil {
		return true
	}
	return elevation.TokenIsElevated == 0
}

