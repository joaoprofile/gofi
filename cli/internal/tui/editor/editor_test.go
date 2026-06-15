package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveEditor_OverrideTakesPrecedence(t *testing.T) {
	t.Setenv("VISUAL", "vim")
	t.Setenv("EDITOR", "nano")
	bin, args := resolveEditor("/usr/bin/cat -n")
	if bin != "/usr/bin/cat" {
		t.Errorf("expected /usr/bin/cat, got %s", bin)
	}
	if len(args) != 1 || args[0] != "-n" {
		t.Errorf("expected [-n], got %v", args)
	}
}

func TestResolveEditor_FallsBackToEnv(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "/bin/sh")
	bin, _ := resolveEditor("")
	if bin != "/bin/sh" {
		t.Errorf("expected /bin/sh from EDITOR, got %s", bin)
	}
}

func TestResolveEditor_PlatformDefault(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("only meaningful on POSIX")
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	bin, _ := resolveEditor("")
	// `vi` is expected to exist on most POSIX systems; if not we just expect
	// an empty result.
	if bin != "" && !strings.HasSuffix(bin, "vi") {
		t.Errorf("unexpected default editor: %s", bin)
	}
}

// TestOpen_RoundTrip uses a tiny shell script as the "editor" so the test
// does not require user interaction.
func TestOpen_RoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script-based editor stub is POSIX-only")
	}
	dir := t.TempDir()
	editorScript := filepath.Join(dir, "fake-editor.sh")
	// Replace whatever was in the file with a fixed content.
	body := "#!/bin/sh\nprintf 'replaced by editor\\n' > \"$1\"\n"
	if err := os.WriteFile(editorScript, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := Open("initial template", editorScript, ".md")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !strings.Contains(out, "replaced by editor") {
		t.Errorf("expected replaced content, got %q", out)
	}
}

func TestOpen_PropagatesEditorFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script-based editor stub is POSIX-only")
	}
	if _, err := Open("", "/bin/false", ".md"); err == nil {
		t.Fatal("expected error from failing editor")
	}
}
