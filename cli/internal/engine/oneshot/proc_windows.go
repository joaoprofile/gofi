//go:build windows

package oneshot

import (
	"os/exec"
	"syscall"
)

func ownGroup(*exec.Cmd) {}

// signalGroup only knows how to kill on Windows.
func signalGroup(cmd *exec.Cmd, _ syscall.Signal) error { return cmd.Process.Kill() }
