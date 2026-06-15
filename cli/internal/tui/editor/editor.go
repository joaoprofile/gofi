// Package editor invokes the user's terminal editor on a temporary file and
// returns the saved contents. Used by `gofi train` to let the user paste a
// markdown buffer or edit an existing topic in place.
package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Open writes initial to a temp file, runs the user's editor on it, then
// reads it back. override forces a specific editor command (split on spaces);
// when "", $VISUAL → $EDITOR → platform default is consulted.
//
// suffix is appended to the temp file name to give editors syntax hints
// (e.g. ".md").
func Open(initial, override, suffix string) (string, error) {
	editor, args := resolveEditor(override)
	if editor == "" {
		return "", errors.New("no editor found; set $VISUAL or $EDITOR, or pass --editor")
	}

	f, err := os.CreateTemp("", "gofi-train-*"+suffix)
	if err != nil {
		return "", fmt.Errorf("create tempfile: %w", err)
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.WriteString(initial); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("write tempfile: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close tempfile: %w", err)
	}

	cmd := exec.Command(editor, append(args, path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q exited with error: %w", editor, err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read tempfile: %w", err)
	}
	return string(out), nil
}

// resolveEditor returns the editor binary and any leading arguments. When
// override is non-empty it takes precedence; otherwise $VISUAL → $EDITOR →
// platform default. Override and env values are split on whitespace so
// commands like "code --wait" work.
func resolveEditor(override string) (string, []string) {
	candidates := []string{
		strings.TrimSpace(override),
		strings.TrimSpace(os.Getenv("VISUAL")),
		strings.TrimSpace(os.Getenv("EDITOR")),
		platformDefault(),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		parts := strings.Fields(c)
		bin := parts[0]
		if _, err := exec.LookPath(bin); err == nil {
			return bin, parts[1:]
		}
		// Allow absolute paths even when LookPath fails (rare on Windows).
		if filepath.IsAbs(bin) {
			if _, err := os.Stat(bin); err == nil {
				return bin, parts[1:]
			}
		}
	}
	return "", nil
}

func platformDefault() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	// vi is the most ubiquitous terminal editor on POSIX systems.
	return "vi"
}
