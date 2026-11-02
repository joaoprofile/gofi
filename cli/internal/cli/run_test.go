package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// fakeSession stands in for Claude Code: the auditor writes a clean verdict
// for the run and iteration its turn names; any other role does nothing.
type fakeSession struct {
	root    string
	prompts *[]string
}

var reVerdictCmd = regexp.MustCompile(`gofi verdict write (\S+) --run (\S+) --iter (\d+)`)

func (f fakeSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	*f.prompts = append(*f.prompts, t.Prompt)
	if strings.HasPrefix(t.Prompt, "/gofi-qa ") {
		m := reVerdictCmd.FindStringSubmatch(t.Prompt)
		iter, _ := strconv.Atoi(m[3])
		v, _ := verdict.Parse([]byte(`{"result":"aprovado"}`), m[1])
		v.Run, v.Iter = m[2], iter
		_, _ = verdict.Write(f.root, v)
	}
	ch := make(chan engine.Event, 1)
	ch <- engine.Done{CostUSD: 0.02, Duration: time.Second}
	close(ch)
	return ch, nil
}
func (fakeSession) Cancel()      {}
func (fakeSession) Close() error { return nil }

func TestRunCarriesAnApprovedChangeToTheLivingSpec(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	var prompts []string
	runPorts.open = func(root string) engine.Session { return fakeSession{root: root, prompts: &prompts} }
	t.Cleanup(func() { runPorts.open = nil })
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  sensors:\n    test: \"true\"\n")

	dir := filepath.Join(root, "specs", "changes", "criar-pedidos")
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\ndesign: n/a\nkeywords: [pedido]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n")

	run := func(cmd func() *cobra.Command, args ...string) (string, error) {
		var out bytes.Buffer
		c := cmd()
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}
	// Not sealed: nothing runs.
	_, err := run(newRunCmd, "criar-pedidos")
	if err == nil || !strings.Contains(err.Error(), "not approved") || len(prompts) != 0 {
		t.Fatalf("an unsealed change ran: %v, %d turns", err, len(prompts))
	}
	if _, err := run(newChangeCmd, "approve", "criar-pedidos"); err != nil {
		t.Fatal(err)
	}

	out, err := run(newRunCmd, "criar-pedidos")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if len(prompts) != 2 || !strings.HasPrefix(prompts[0], "/gofi-eng ") || !strings.HasPrefix(prompts[1], "/gofi-qa ") {
		t.Errorf("turns: %q", prompts)
	}
	if !strings.Contains(out, "accepted") || !strings.Contains(out, "specs/changes/archive/") {
		t.Errorf("output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "pedidos", "spec.md")); err != nil {
		t.Error("the accepted change was not archived into the living spec")
	}

	out, _ = run(newPsCmd)
	if !strings.Contains(out, "criar-pedidos") || !strings.Contains(out, "accepted") {
		t.Errorf("ps:\n%s", out)
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".gofi", "runs"))
	var id string
	for _, e := range entries {
		if e.IsDir() {
			id = e.Name()
		}
	}
	out, err = run(newLogsCmd, id)
	if err != nil || !strings.Contains(out, "run.end") || !strings.Contains(out, "policy → accepted") {
		t.Errorf("logs: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".gofi", "runs", id, "agents", "implement-1.ndjson")); err != nil {
		t.Error("no transcript of the implementation turn")
	}
	if _, err := run(newCancelCmd, id); err == nil {
		t.Error("cancelling an ended run passed")
	}
	var exit *ExitCodeError
	if _, err := run(newRunCmd); !errors.As(err, &exit) && err == nil {
		t.Error("run with nothing named passed")
	}
}
