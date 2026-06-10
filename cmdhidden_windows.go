//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func hideCmdWindow(cmd *exec.Cmd, show bool) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: !show}
}
