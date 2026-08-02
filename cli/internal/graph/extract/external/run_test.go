package external

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// helper turns the test binary itself into a fake extractor, so the process
// boundary is exercised for real without needing a toolchain at test time.
func helper(behavior string) Spec {
	return Spec{
		Language: "fake",
		Path:     os.Args[0],
		// Everything after "--" is left alone by the flag package, so the
		// --root and --mode that Run appends reach the helper unparsed.
		Args: []string{"-test.run=^TestHelperExtractor$", "--", "gofi-helper", behavior},
	}
}

// TestHelperExtractor is a no-op in a normal run; it only acts when Run
// re-executes this binary through helper(). It exits without returning so the
// testing framework never gets to print "PASS" into the extractor's stdout.
func TestHelperExtractor(t *testing.T) {
	i := slices.Index(os.Args, "gofi-helper")
	if i < 0 {
		t.Skip("not running as a fake extractor")
	}
	behavior := os.Args[i+1]
	out, errOut := os.Stdout, os.Stderr

	switch behavior {
	case "ok":
		fmt.Fprintln(out, `{"rec":"header","schema":"gofi-graph/v1","language":"fake","module":"m","tool":"fake 1.0"}`)
		fmt.Fprintln(out, `{"rec":"node","id":"fake:A","kind":"class","name":"A"}`)
		fmt.Fprintln(out, `{"rec":"node","id":"fake:B","kind":"class","name":"B"}`)
		fmt.Fprintln(out, `{"rec":"edge","from":"fake:A","to":"fake:B","rel":"uses","conf":0.5}`)
	case "echo-args":
		fmt.Fprintln(out, `{"rec":"header","schema":"gofi-graph/v1","language":"fake"}`)
		fmt.Fprintf(out, "{\"rec\":\"diag\",\"severity\":\"info\",\"msg\":%q}\n", strings.Join(os.Args[i+2:], " "))
	case "no-tool":
		fmt.Fprintln(out, `{"rec":"header","schema":"gofi-graph/v1","language":"fake"}`)
	case "boom":
		fmt.Fprintln(errOut, "java.lang.NullPointerException")
		os.Exit(3)
	case "silent":
		// Exits 0 having said nothing at all.
	case "garbage":
		fmt.Fprintln(out, `{"rec":"header","schema":"gofi-graph/v1","language":"fake"}`)
		fmt.Fprintln(out, "not json")
		fmt.Fprintln(errOut, "warming up")
	case "hang":
		fmt.Fprintln(out, `{"rec":"header","schema":"gofi-graph/v1","language":"fake"}`)
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func runHelper(t *testing.T, behavior string, opt RunOptions) (*Result, error) {
	t.Helper()
	if opt.Root == "" {
		opt.Root = t.TempDir()
	}
	return Run(t.Context(), helper(behavior), opt)
}

func TestRun(t *testing.T) {
	res, err := runHelper(t, "ok", RunOptions{Limits: DefaultLimits()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Graph.Nodes) != 2 || len(res.Graph.Edges) != 1 {
		t.Errorf("got %d nodes and %d edges, want 2 and 1", len(res.Graph.Nodes), len(res.Graph.Edges))
	}
	if res.Graph.Tool != "fake 1.0" {
		t.Errorf("tool = %q, want the one the header declared", res.Graph.Tool)
	}
}

// An extractor that names no tool is identified by the executable that ran, so
// a graph can always be traced back to what produced it.
func TestRunFallsBackToPathAsTool(t *testing.T) {
	res, err := runHelper(t, "no-tool", RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Graph.Tool != os.Args[0] {
		t.Errorf("tool = %q, want %q", res.Graph.Tool, os.Args[0])
	}
}

func TestRunPassesRootAndMode(t *testing.T) {
	root := t.TempDir()
	res, err := runHelper(t, "echo-args", RunOptions{Root: root, Deep: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Diagnostics) != 1 {
		t.Fatalf("helper did not echo its arguments: %+v", res.Diagnostics)
	}
	got := res.Diagnostics[0].Message
	for _, want := range []string{"--root " + root, "--mode deep"} {
		if !strings.Contains(got, want) {
			t.Errorf("arguments %q missing %q", got, want)
		}
	}

	res, err = runHelper(t, "echo-args", RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Diagnostics[0].Message, "--mode fast") {
		t.Errorf("without Deep the mode should be fast, got %q", res.Diagnostics[0].Message)
	}
}

func TestRunReportsExitStatusWithStderr(t *testing.T) {
	_, err := runHelper(t, "boom", RunOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}
	// The exit status explains the failure better than the truncated stream
	// does, so it has to come first, and the stderr tail has to come along.
	for _, want := range []string{"codigo 3", "NullPointerException", "stderr do extractor"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestRunRejectsSilentExtractor(t *testing.T) {
	_, err := runHelper(t, "silent", RunOptions{})
	if err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("err = %v, want a complaint about the missing header", err)
	}
}

func TestRunReportsDecodeFailure(t *testing.T) {
	_, err := runHelper(t, "garbage", RunOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"JSON invalido", "warming up"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestRunTimesOut(t *testing.T) {
	start := time.Now()
	_, err := runHelper(t, "hang", RunOptions{Timeout: 200 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "tempo limite") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v to give up on a hung extractor", d)
	}
}

func TestRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := Run(ctx, helper("ok"), RunOptions{Root: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "cancelado") {
		t.Fatalf("err = %v, want a cancellation", err)
	}
}

func TestRunWithoutPath(t *testing.T) {
	_, err := Run(t.Context(), Spec{Language: "java"}, RunOptions{})
	if err == nil || !strings.Contains(err.Error(), "caminho") {
		t.Fatalf("err = %v, want a complaint about the missing path", err)
	}
}
