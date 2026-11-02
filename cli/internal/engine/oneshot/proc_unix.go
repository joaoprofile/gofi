//go:build !windows

package oneshot

import (
	"os/exec"
	"syscall"
)

// ownGroup starts the engine in a process group of its own, so a cancel ends
// the commands its tools started too.
func ownGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// signalGroup sends sig to the engine and everything it started.
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	return syscall.Kill(-cmd.Process.Pid, sig)
}
