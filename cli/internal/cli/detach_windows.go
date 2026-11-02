//go:build windows

package cli

import "syscall"

// detached puts a background supervisor in a process group of its own, with
// no console: closing the terminal that started it does not stop it.
func detached() *syscall.SysProcAttr {
	const detachedProcess = 0x00000008
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}
