//go:build windows

package system

import (
	"os/exec"
	"syscall"
)

func hideCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
