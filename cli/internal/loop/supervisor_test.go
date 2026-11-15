package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Live is the runs a supervisor holds right now: the same ones List calls
// running or blocked, and none of the others.
func TestLiveIsWhatListCallsRunningOrBlocked(t *testing.T) {
	root := t.TempDir()
	put := func(run, node string) {
		t.Helper()
		dir := Dir(root, run)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(State{Schema: StateSchema, Run: run, Flow: "delivery", Change: run, Node: node, Taken: map[string]int{}, Updated: time.Now()})
		if err := os.WriteFile(filepath.Join(dir, "state.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("r-ended", "")
	put("r-paused", "implement")
	put("r-live", "implement")
	release, err := Acquire(root, "r-live")
	if err != nil {
		t.Fatal(err)
	}
	// A lock left by a process that is gone is an interrupted run, not a live one.
	put("r-gone", "implement")
	gone, _ := json.Marshal(Lock{PID: 1 << 30, Started: time.Now()})
	if err := os.WriteFile(filepath.Join(Dir(root, "r-gone"), LockFile), gone, 0o644); err != nil {
		t.Fatal(err)
	}

	live, err := Live(root)
	if err != nil || len(live) != 1 || live[0].State.Run != "r-live" || live[0].Liveness != Running {
		t.Fatalf("Live = %+v, %v", live, err)
	}
	all, _ := List(root)
	var want []string
	for _, info := range all {
		if info.Liveness == Running || info.Liveness == Blocked {
			want = append(want, info.State.Run)
		}
	}
	if len(all) != 4 || len(want) != 1 || want[0] != "r-live" {
		t.Errorf("List disagrees: %v of %d", want, len(all))
	}

	release()
	if live, _ = Live(root); len(live) != 0 {
		t.Errorf("a run nobody holds is not live: %+v", live)
	}
	if live, err = Live(t.TempDir()); err != nil || live != nil {
		t.Errorf("a project with no runs has none live: %v %v", live, err)
	}
}
