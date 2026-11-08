package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// A conflict the person solved by hand stops being shown as one once the
// base holds the branch; one still open stays.
func TestALandingDoneByHandIsRecognised(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "--quiet", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "--quiet", "-m", "a")
	git("branch", "gofi/done")
	git("switch", "--quiet", "-c", "gofi/open")
	if err := os.WriteFile(filepath.Join(root, "b"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "--quiet", "-m", "b")
	git("switch", "--quiet", "main")
	save := func(run, branch string) {
		if err := os.MkdirAll(loop.Dir(root, run), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := loop.SaveState(root, &loop.State{Schema: loop.StateSchema, Run: run, Change: run, Outcome: "accepted",
			Landing: conflict, Base: "main", Branch: branch, Taken: map[string]int{}}); err != nil {
			t.Fatal(err)
		}
	}
	save("r-1", "gofi/done")
	save("r-2", "gofi/open")
	settleLandings(context.Background(), root)
	if s, _ := loop.LoadState(root, "r-1"); s.Landing != landed {
		t.Errorf("landed by hand: %s", s.Landing)
	}
	if s, _ := loop.LoadState(root, "r-2"); s.Landing != conflict {
		t.Errorf("still open: %s", s.Landing)
	}
}
