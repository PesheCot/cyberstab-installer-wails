//go:build !windows

package db

import "os/exec"

func hideCmd(cmd *exec.Cmd) {}
