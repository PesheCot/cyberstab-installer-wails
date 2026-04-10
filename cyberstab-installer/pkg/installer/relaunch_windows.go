//go:build windows

package installer

import (
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func tryRelaunchAsAdmin(args []string) bool {
	if len(args) == 0 {
		return false
	}
	exe := args[0]
	params := ""
	if len(args) > 1 {
		params = joinWindowsCommandLineArgs(args[1:])
	}

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	var argPtr *uint16
	if params != "" {
		argPtr, _ = syscall.UTF16PtrFromString(params)
	}

	// showCmd: 1 = SW_SHOWNORMAL
	if err := windows.ShellExecute(0, verb, file, argPtr, nil, 1); err != nil {
		return false
	}
	return true
}

func joinWindowsCommandLineArgs(args []string) string {
	// Minimal quoting for CreateProcess-style command lines:
	// quote args with spaces or quotes, escape internal quotes.
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "" {
			out = append(out, "\"\"")
			continue
		}
		if strings.ContainsAny(a, " \t\"") {
			escaped := strings.ReplaceAll(a, `"`, `\"`)
			out = append(out, `"`+escaped+`"`)
		} else {
			out = append(out, a)
		}
	}
	return strings.Join(out, " ")
}

