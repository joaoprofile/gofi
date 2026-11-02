package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowCheck(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := newFlowCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	if out, err := run("check"); err != nil || !strings.Contains(out, "gofi:delivery") || !strings.Contains(out, "valid") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	writeFile(t, filepath.Join(root, ".claude", "flows", "loop.yaml"), `schema: gofi-flow/v1
name: loop
start: a
nodes:
  a: {kind: agent, role: gofi-eng}
  b: {kind: policy}
  z: {kind: terminal, outcome: accepted}
edges:
  - {from: a, to: b}
  - {from: b, to: z, when: {verdict: clean}}
  - {from: b, to: a}
`)
	out, err := run("check")
	var exit *ExitCodeError
	if !errors.As(err, &exit) || exit.Code != 1 || !strings.Contains(out, "loop forever") {
		t.Errorf("err=%v\n%s", err, out)
	}
}
