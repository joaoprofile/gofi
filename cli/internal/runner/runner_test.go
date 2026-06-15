package runner

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell required for these tests")
	}
}

func makeRunner(tasks map[string]config.TestTask, pre, post []string) (*Runner, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	r := &Runner{
		Cfg: &config.TestSection{
			Default: "",
			Hooks:   config.TestHooks{Pre: pre, Post: post},
			Tasks:   tasks,
		},
		Stdout: out,
		Stderr: errBuf,
	}
	return r, out, errBuf
}

func TestRunner_RunSimple(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"hello": {Run: "printf hi"},
	}, nil, nil)
	if err := r.Run("hello", nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); got != "hi" {
		t.Errorf("expected hi, got %q", got)
	}
}

func TestRunner_NeedsResolved(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"a": {Run: "printf 'A '"},
		"b": {Run: "printf 'B '", Needs: []string{"a"}},
		"c": {Run: "printf C", Needs: []string{"b"}},
	}, nil, nil)
	if err := r.Run("c", nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); got != "A B C" {
		t.Errorf("expected 'A B C', got %q", got)
	}
}

func TestRunner_Cycle(t *testing.T) {
	r, _, _ := makeRunner(map[string]config.TestTask{
		"a": {Run: "echo a", Needs: []string{"b"}},
		"b": {Run: "echo b", Needs: []string{"a"}},
	}, nil, nil)
	err := r.Run("a", nil)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestRunner_UnknownTask(t *testing.T) {
	r, _, _ := makeRunner(map[string]config.TestTask{
		"a": {Run: "echo a"},
	}, nil, nil)
	if err := r.Run("nope", nil); err == nil {
		t.Fatal("expected unknown task error")
	}
}

func TestRunner_TaskFailurePropagates(t *testing.T) {
	skipOnWindows(t)
	r, _, _ := makeRunner(map[string]config.TestTask{
		"fail": {Run: "exit 7"},
	}, nil, nil)
	err := r.Run("fail", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task \"fail\" failed") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestRunner_HooksRunBeforeAndAfter(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"work": {Run: "printf 'W '"},
	}, []string{"printf 'PRE '"}, []string{"printf POST"})
	if err := r.Run("work", nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); got != "PRE W POST" {
		t.Errorf("expected 'PRE W POST', got %q", got)
	}
}

func TestRunner_PreHookFailureAborts(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"work": {Run: "printf W"},
	}, []string{"exit 1"}, nil)
	err := r.Run("work", nil)
	if err == nil {
		t.Fatal("expected pre-hook to abort")
	}
	if strings.Contains(out.String(), "W") {
		t.Errorf("task should not have run; got %q", out.String())
	}
}

func TestRunner_PostHookRunsEvenWhenTaskFails(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"work": {Run: "exit 1"},
	}, nil, []string{"printf POST"})
	err := r.Run("work", nil)
	if err == nil {
		t.Fatal("expected task error")
	}
	if got := out.String(); got != "POST" {
		t.Errorf("expected POST in output, got %q", got)
	}
}

func TestRunner_ExtraArgsForwarded(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"echo": {Run: "printf '%s '"},
	}, nil, nil)
	if err := r.Run("echo", []string{"hello", "world", "with space"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); got != "hello world with space " {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRunner_ExtraArgsOnlyOnRequestedTask(t *testing.T) {
	skipOnWindows(t)
	r, out, _ := makeRunner(map[string]config.TestTask{
		"a": {Run: "printf '%s' deps"},
		"b": {Run: "printf '%s' top", Needs: []string{"a"}},
	}, nil, nil)
	if err := r.Run("b", []string{" extras"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "deps") || !strings.Contains(got, "top extras") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRunner_List(t *testing.T) {
	r, _, _ := makeRunner(map[string]config.TestTask{
		"unit":  {Desc: "unit tests", Run: "x"},
		"cover": {Desc: "cover", Run: "x", Needs: []string{"unit"}},
	}, nil, nil)
	infos := r.List()
	if len(infos) != 2 {
		t.Errorf("expected 2 entries, got %d", len(infos))
	}
}

func TestShellQuote_Posix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only assertions")
	}
	cases := map[string]string{
		"foo":     "'foo'",
		"with sp": "'with sp'",
		"a'b":     `'a'\''b'`,
		"":        "''",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
