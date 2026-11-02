package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// LockFile marks a run a supervisor is carrying out: its pid, so another
// terminal can tell a live run from one that was interrupted, and cancel it.
const LockFile = "supervisor.lock"

// Lock is the supervisor's claim on a run.
type Lock struct {
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
}

// Acquire claims a run for this process. It refuses when a live supervisor
// already holds it; a lock left by a process that is gone is taken over.
func Acquire(root, run string) (release func(), err error) {
	dir := Dir(root, run)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if l, ok := ReadLock(root, run); ok && alive(l.PID) && l.PID != os.Getpid() {
		return nil, fmt.Errorf("a execução %s já está rodando (pid %d)", run, l.PID)
	}
	data, _ := json.Marshal(Lock{PID: os.Getpid(), Started: time.Now().UTC()})
	p := filepath.Join(dir, LockFile)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(p) }, nil
}

// ReadLock reads a run's lock, when there is one.
func ReadLock(root, run string) (Lock, bool) {
	data, err := os.ReadFile(filepath.Join(Dir(root, run), LockFile))
	if err != nil {
		return Lock{}, false
	}
	var l Lock
	if json.Unmarshal(data, &l) != nil {
		return Lock{}, false
	}
	return l, true
}

// Liveness of a run, as gofi ps shows it.
const (
	Running     = "rodando"
	Interrupted = "interrompida" // a supervisor held it and is gone: resume it
	Ended       = "encerrada"
	Paused      = "parada" // stopped at a node, nobody carrying it: resume it
)

// Info is one run as listed.
type Info struct {
	State    *State
	Liveness string
	PID      int
}

// List reads every run that has a state, newest first.
func List(root string) ([]Info, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(RunsDir)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := LoadState(root, e.Name())
		if err != nil {
			continue
		}
		info := Info{State: s}
		l, locked := ReadLock(root, e.Name())
		switch {
		case s.Node == "":
			info.Liveness = Ended
		case locked && alive(l.PID):
			info.Liveness, info.PID = Running, l.PID
		case locked:
			info.Liveness = Interrupted
		default:
			info.Liveness = Paused
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].State.Updated.After(out[j].State.Updated) })
	return out, nil
}

// Alive reports whether the process holding a lock is still there.
func Alive(pid int) bool { return alive(pid) }

// Cancel asks the supervisor of a live run to stop: it saves the state at the
// node it is in, and the run can be resumed.
func Cancel(root, run string) error {
	l, ok := ReadLock(root, run)
	if !ok || !alive(l.PID) {
		return fmt.Errorf("a execução %s não está rodando", run)
	}
	p, err := os.FindProcess(l.PID)
	if err != nil {
		return err
	}
	return interrupt(p)
}
