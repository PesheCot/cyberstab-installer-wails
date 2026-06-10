//go:build !windows

package system

import "os/exec"

func hideCmd(cmd *exec.Cmd) {}
