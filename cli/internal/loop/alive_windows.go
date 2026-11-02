//go:build windows

package loop

import (
	"os"

	"golang.org/x/sys/windows"
)

// alive reports whether a process is still there.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

// interrupt stops a supervisor. Windows has no interrupt to send another
// process; the state was checkpointed at the last node, so the run resumes
// from there.
func interrupt(p *os.Process) error { return p.Kill() }
