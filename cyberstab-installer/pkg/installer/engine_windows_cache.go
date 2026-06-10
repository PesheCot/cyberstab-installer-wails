//go:build windows

package installer

import "syscall"

func refreshWindowsIconCache() {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shChangeNotify := shell32.NewProc("SHChangeNotify")
	const (
		shcneAssocChanged = 0x08000000
		shcnfIDList       = 0x0000
	)
	_, _, _ = shChangeNotify.Call(uintptr(shcneAssocChanged), uintptr(shcnfIDList), 0, 0)
	_ = runHidden("ie4uinit.exe", "-show")
}
