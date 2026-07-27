package extensions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/vsix"
)

// Editor is one VSCode-family CLI that can install a .vsix.
type Editor struct {
	// Command is the name probed on PATH.
	Command string
	// Label is what the user sees in output.
	Label string
	// Path is the resolved absolute path, filled by DetectEditors.
	Path string
}

// knownEditors is the probe order. VSCode first because it is what `gofi init`
// targets; the forks accept the same `--install-extension` flag, so supporting
// them costs one line each.
var knownEditors = []Editor{
	{Command: "code", Label: "VS Code"},
	{Command: "cursor", Label: "Cursor"},
	{Command: "code-insiders", Label: "VS Code Insiders"},
	{Command: "codium", Label: "VSCodium"},
	{Command: "windsurf", Label: "Windsurf"},
}

// DetectEditors returns the editor CLIs present on PATH, in probe order.
func DetectEditors() []Editor {
	var found []Editor
	for _, e := range knownEditors {
		path, err := exec.LookPath(e.Command)
		if err != nil {
			continue
		}
		e.Path = path
		found = append(found, e)
	}
	return found
}

// LookupEditor resolves a single editor by command name.
func LookupEditor(command string) (Editor, error) {
	for _, e := range knownEditors {
		if e.Command != command {
			continue
		}
		path, err := exec.LookPath(command)
		if err != nil {
			return Editor{}, fmt.Errorf("%s (%s) not found on PATH", e.Label, command)
		}
		e.Path = path
		return e, nil
	}
	// Not a name we know, but the user asked for it explicitly — if it's on
	// PATH, try it. Every VSCode fork takes the same flags.
	path, err := exec.LookPath(command)
	if err != nil {
		return Editor{}, fmt.Errorf("%s not found on PATH", command)
	}
	return Editor{Command: command, Label: command, Path: path}, nil
}

// Result records what happened for one editor.
type Result struct {
	Editor Editor
	Err    error
	Output string
}

// InstallEmbedded installs the packaged GOFI AI extension into each editor.
//
// The .vsix is written to a temp file first because the editor CLIs take a
// path, not stdin. Every editor is attempted even if an earlier one fails —
// a broken Cursor install shouldn't stop VSCode from getting the extension.
func InstallEmbedded(ctx context.Context, editors []Editor) ([]Result, vsix.Manifest, error) {
	data, manifest, err := Embedded()
	if err != nil {
		return nil, manifest, err
	}

	dir, err := os.MkdirTemp("", "gofi-ext-")
	if err != nil {
		return nil, manifest, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	vsixPath := filepath.Join(dir, manifest.VSIXName())
	if err := os.WriteFile(vsixPath, data, 0o644); err != nil {
		return nil, manifest, fmt.Errorf("write vsix: %w", err)
	}

	results := make([]Result, 0, len(editors))
	for _, e := range editors {
		out, err := runEditor(ctx, e, "--install-extension", vsixPath, "--force")
		results = append(results, Result{Editor: e, Err: err, Output: out})
	}
	return results, manifest, nil
}

// Installed reports the version of the given extension id in an editor, or ""
// when it is not installed.
func Installed(ctx context.Context, e Editor, id string) (string, error) {
	out, err := runEditor(ctx, e, "--list-extensions", "--show-versions")
	if err != nil {
		return "", err
	}
	prefix := strings.ToLower(id) + "@"
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return line[len(prefix):], nil
		}
	}
	return "", nil
}

// runEditor invokes an editor CLI, returning its combined output. The timeout
// guards against an editor CLI that waits on a window that will never appear.
func runEditor(ctx context.Context, e Editor, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.Path, args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() != nil {
			return text, fmt.Errorf("%s timed out", e.Command)
		}
		if text != "" {
			return text, fmt.Errorf("%s %s: %w — %s", e.Command, strings.Join(args, " "), err, firstLine(text))
		}
		return text, fmt.Errorf("%s %s: %w", e.Command, strings.Join(args, " "), err)
	}
	return text, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i != -1 {
		return s[:i]
	}
	return s
}
