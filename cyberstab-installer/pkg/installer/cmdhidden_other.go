//go:build !windows

package installer

import "os/exec"

func hideCmd(cmd *exec.Cmd) {}

func hideCmdDetached(cmd *exec.Cmd) {}
