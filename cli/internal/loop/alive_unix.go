//go:build !windows

package loop

import (
	"os"
	"syscall"
)

// alive reports whether a process is still there: signal 0 checks without
// delivering anything.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// interrupt asks a supervisor to stop as Ctrl+C would.
func interrupt(p *os.Process) error { return p.Signal(os.Interrupt) }
