//go:build !windows

package cli

import "syscall"

// detached puts a background supervisor in a session of its own: closing the
// terminal that started it does not stop it.
func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
