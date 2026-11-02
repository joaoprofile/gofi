package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// gofi answer finds the run waiting at its gate, says in a script what it
// waits on, and its answer is what the gate returns.
func TestAnswerUnblocksTheRunWaitingAtItsGate(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	release, err := loop.Acquire(root, "r-1")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	state, _ := json.Marshal(loop.State{Schema: loop.StateSchema, Run: "r-1", Change: "criar-pedidos", Node: "decide", Taken: map[string]int{}})
	writeFile(t, filepath.Join(loop.Dir(root, "r-1"), "state.json"), string(state))

	answered := make(chan map[string]string, 1)
	g := &loop.FileGate{Root: root, Run: "r-1", Timeout: time.Minute, Poll: 5 * time.Millisecond}
	go func() {
		a, _ := g.Ask(context.Background(), loop.Question{Agent: "implement-1", Questions: []intake.ElicitQuest{
			{ID: "q1", Text: "Arredonda?", Options: []string{"sim", "não"}, Recommended: "sim"}}})
		answered <- a
	}()
	for range 200 {
		if _, ok := loop.OpenGate(root, "r-1"); ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	answer := func(args ...string) (string, error) {
		var out bytes.Buffer
		c := newAnswerCmd()
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		c.SetIn(strings.NewReader(""))
		err := c.Execute()
		return out.String(), err
	}
	out, err := answer()
	if err == nil || !strings.Contains(out, "Arredonda?") || !strings.Contains(out, "sim | não") {
		t.Errorf("a script was not told what the run waits on: %v\n%s", err, out)
	}
	if _, err := answer("--set", "q1=talvez"); err == nil {
		t.Error("an answer that is not an option passed")
	}
	if out, err := answer("--set", "q1=não"); err != nil {
		t.Fatalf("answer: %v\n%s", err, out)
	}
	select {
	case a := <-answered:
		if a["q1"] != "não" {
			t.Errorf("the gate got %v", a)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the gate never got the answer")
	}
}
