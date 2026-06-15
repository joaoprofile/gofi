package aiclient

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestForHost_Claude(t *testing.T) {
	c, err := ForHost("claude-vscode")
	if err != nil {
		t.Fatalf("ForHost: %v", err)
	}
	if c.Name() != "claude-vscode" {
		t.Errorf("expected name claude-vscode, got %s", c.Name())
	}
}

func TestForHost_Unknown(t *testing.T) {
	if _, err := ForHost("cursor"); err == nil {
		t.Error("expected error for unknown host")
	}
}

func TestClaude_Available_Smoke(t *testing.T) {
	// We cannot assert true/false reliably on every test machine, but the
	// function must not panic and must agree with PATH lookup.
	c := newClaudeClient()
	_ = c.Available()
}

func TestClaude_Invoke_RoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script-based stub is POSIX-only")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "fake-claude.sh")
	body := "#!/bin/sh\n" +
		// pipe stdin to stdout, so the test asserts the prompt arrived
		"cat\n" +
		// emit a marker so we can detect that the stub ran
		"echo '---STUB-OK---'\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	c := &claudeClient{bin: stub}

	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Invoke(ctx, "hello agent", &out, &out); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	got := out.String()
	if !contains(got, "hello agent") {
		t.Errorf("stub did not receive prompt; got: %q", got)
	}
	if !contains(got, "STUB-OK") {
		t.Errorf("stub did not run; got: %q", got)
	}
}

func TestClaude_Invoke_PropagatesNonZero(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/bin/false is POSIX-only")
	}
	c := &claudeClient{bin: "/bin/false"}
	err := c.Invoke(context.Background(), "anything", &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected non-nil error from /bin/false")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
