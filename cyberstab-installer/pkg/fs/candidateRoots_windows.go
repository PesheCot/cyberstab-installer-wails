//go:build windows

package fs

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

const (
	driveUnknown   = 0
	driveNoRootDir = 1
	driveRemovable = 2
	driveFixed     = 3
	driveRemote    = 4
	driveCDROM     = 5
	driveRAMDisk   = 6
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDrv = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW = kernel32.NewProc("GetDriveTypeW")
)

func candidateRoots() []string {
	// Preferred: detect actual USB disks (BusType=USB) and map to drive letters.
	// This avoids scanning fixed disks that may contain installed files.
	if roots := usbDriveRootsPowerShell(); len(roots) > 0 {
		return roots
	}

	mask, _, _ := procGetLogicalDrv.Call()
	if mask == 0 {
		// Fallback: old behaviour, but skip C:
		var out []string
		for c := byte('D'); c <= 'Z'; c++ {
			out = append(out, fmt.Sprintf("%c:\\", c))
		}
		return out
	}

	var out []string
	for i := 0; i < 26; i++ {
		if (mask & (1 << uint(i))) == 0 {
			continue
		}
		letter := byte('A' + i)
		root := fmt.Sprintf("%c:\\", letter)

		// Determine drive type.
		pRoot, _ := syscall.UTF16PtrFromString(root)
		dt, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(pRoot)))

		// Fallback: only removable / CD-ROM (user requested: only flash drives).
		if dt == driveRemovable || dt == driveCDROM {
			out = append(out, root)
		}
	}

	return out
}

func usbDriveRootsPowerShell() []string {
	// Collect DriveLetter of partitions that belong to USB disks.
	// Output lines like: F
	// Try Storage cmdlets first (Windows 10+), then WMI/CIM fallback (works on older systems too).
	scripts := []string{
		strings.Join([]string{
			"$ErrorActionPreference='SilentlyContinue'",
			"$letters = Get-Disk | Where-Object { $_.BusType -eq 'USB' } | ForEach-Object {",
			"  Get-Partition -DiskNumber $_.Number | Where-Object { $_.DriveLetter } | Select-Object -ExpandProperty DriveLetter",
			"}",
			"$letters | Sort-Object -Unique",
		}, "; "),
		strings.Join([]string{
			"$ErrorActionPreference='SilentlyContinue'",
			// Win32_DiskDrive -> Win32_DiskPartition -> Win32_LogicalDisk, return DeviceID like 'F:'
			"$dev = Get-CimInstance Win32_DiskDrive | Where-Object { $_.InterfaceType -eq 'USB' }",
			"if (-not $dev) { exit 0 }",
			"$ids = foreach ($d in $dev) {",
			"  $parts = Get-CimAssociatedInstance -InputObject $d -ResultClassName Win32_DiskPartition",
			"  foreach ($p in $parts) {",
			"    Get-CimAssociatedInstance -InputObject $p -ResultClassName Win32_LogicalDisk | Select-Object -ExpandProperty DeviceID",
			"  }",
			"}",
			"$ids | Sort-Object -Unique",
		}, "; "),
	}

	for _, script := range scripts {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		var out bytes.Buffer
		cmd.Stdout = &out
		_ = cmd.Run()

		if roots := parseDriveRoots(out.String()); len(roots) > 0 {
			return roots
		}
	}

	return nil
}

func parseDriveRoots(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	set := map[string]struct{}{}
	for _, l := range lines {
		l = strings.TrimSpace(strings.Trim(l, "\r"))
		if l == "" {
			continue
		}
		// Accept either "F" or "F:".
		l = strings.TrimSuffix(l, ":")
		letter := strings.ToUpper(l[:1])
		if letter < "A" || letter > "Z" {
			continue
		}
		set[letter+":\\"] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	roots := make([]string, 0, len(set))
	for r := range set {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	return roots
}

