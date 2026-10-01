package leancall

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeClaude writes a claude that records its arguments and working directory
// and prints the given reply.
func fakeClaude(t *testing.T, reply string) (exe, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	exe = filepath.Join(dir, "claude")
	script := "#!/bin/sh\npwd > " + argsFile + ".pwd\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > " + argsFile + "\ncat <<'EOF'\n" + reply + "\nEOF\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe, argsFile
}

func TestCallReturnsTheValidatedAnswerAndItsCost(t *testing.T) {
	exe, argsFile := fakeClaude(t, `{"subtype":"success","structured_output":{"intent":"change"},"total_cost_usd":0.0048,"duration_ms":5100,"usage":{"input_tokens":1350,"output_tokens":480}}`)
	out, spend, err := Claude{Executable: exe}.Call(context.Background(), Request{
		System: "defs", Prompt: "pedido", Schema: []byte(`{"type":"object"}`), Model: "haiku",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"intent":"change"}` || spend.USD != 0.0048 || spend.Millis != 5100 || spend.InputTokens != 1350 {
		t.Errorf("out=%s spend=%+v", out, spend)
	}
	b, _ := os.ReadFile(argsFile)
	args := strings.Split(strings.TrimSpace(string(b)), "\n")
	for _, want := range []string{"--tools", "--strict-mcp-config", "--no-session-persistence", "--json-schema", "--max-budget-usd", "--effort"} {
		if !slices.Contains(args, want) {
			t.Errorf("the lean call lacks %s: %v", want, args)
		}
	}
	// It runs where no project file can be found.
	pwd, _ := os.ReadFile(argsFile + ".pwd")
	if !strings.Contains(string(pwd), "gofi-leancall-") {
		t.Errorf("ran in %s", pwd)
	}
}

func TestCallRejectsAnAnswerThatIsNotValidated(t *testing.T) {
	for _, reply := range []string{
		`{"subtype":"error_max_structured_output_retries","total_cost_usd":0.01}`,
		`{"subtype":"success","result":"texto livre"}`,
		`not json`,
	} {
		exe, _ := fakeClaude(t, reply)
		if _, _, err := (Claude{Executable: exe}).Call(context.Background(), Request{Model: "haiku", Schema: []byte("{}")}); err == nil {
			t.Errorf("%q should be an error", reply)
		}
	}
}
