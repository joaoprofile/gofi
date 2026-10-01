//go:build !windows

package claude

import (
	"os/exec"
	"syscall"
)

// ownGroup starts the engine in a process group of its own, so a cancel can
// end what it started too — a shell command a tool is running would otherwise
// keep its output open, and the turn would never end.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalGroup sends sig to the engine and everything it started.
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	return syscall.Kill(-cmd.Process.Pid, sig)
}
