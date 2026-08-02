// Package githooks installs the hooks that keep gofi's derived files in step
// with the code.
//
// A repository rarely has hooks to spare: husky, lefthook and hand-written
// scripts all want the same files. So gofi never owns a hook file — it owns a
// block inside one, delimited by markers, and leaves everything around the
// block alone. Installing twice rewrites the block; uninstalling removes it and
// gives the file back exactly as it was found.
package githooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Markers delimit the region gofi owns inside a hook script.
const (
	Begin = "# >>> gofi >>>"
	End   = "# <<< gofi <<<"
)

// Managed are the hooks gofi writes to.
//
// pre-commit leads because gofi's derived files are tracked: regenerating one
// there puts it in the same commit as the code it describes. Doing it after the
// commit instead would leave the working tree permanently dirty.
//
// post-checkout and post-merge are the repair pass. Both normally do nothing —
// the file came out of git already matching the code — but a merge that
// resolved a generated file line by line produces something no build would ever
// emit, and that is where it gets fixed.
var Managed = []string{"pre-commit", "post-checkout", "post-merge"}

// Action is what Install or Uninstall did to one hook.
type Action string

const (
	Created   Action = "created"
	Updated   Action = "updated"
	Unchanged Action = "unchanged"
	Removed   Action = "removed"
)

// Result is the outcome for one hook.
type Result struct {
	Hook   string
	Path   string
	Action Action
}

// Dir is the directory the repository actually reads hooks from. It is asked of
// git rather than assumed, because core.hooksPath moves it — which is exactly
// what husky does, and writing to .git/hooks there would be a silent no-op.
func Dir(projectRoot string) (string, error) {
	cmd := exec.Command("git", "-C", projectRoot, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s nao parece um repositorio git: %w", projectRoot, err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("git nao informou o diretorio de hooks de %s", projectRoot)
	}
	return dir, nil
}

// Install writes each body into the gofi block of the hook that keys it. A body
// is the shell to run, without the markers and without a shebang; a managed
// hook with no body is left untouched.
func Install(projectRoot string, bodies map[string]string) ([]Result, error) {
	dir, err := Dir(projectRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(Managed))
	for _, hook := range Managed {
		body, ok := bodies[hook]
		if !ok {
			continue
		}
		r, err := installOne(filepath.Join(dir, hook), hook, body)
		if err != nil {
			return results, fmt.Errorf("hook %s: %w", hook, err)
		}
		results = append(results, r)
	}
	return results, nil
}

func installOne(path, hook, body string) (Result, error) {
	res := Result{Hook: hook, Path: path}
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return res, err
	}

	block := Begin + "\n" + strings.TrimRight(body, "\n") + "\n" + End
	var next string
	switch {
	case err != nil:
		next = "#!/bin/sh\n" + block + "\n"
		res.Action = Created
	default:
		next = replaceBlock(string(old), block)
		res.Action = Updated
	}
	if string(old) == next {
		res.Action = Unchanged
		return res, nil
	}
	return res, os.WriteFile(path, []byte(next), 0o755)
}

// Uninstall removes the gofi block from every managed hook. A hook left holding
// nothing but a shebang is deleted, so uninstalling is not detectable
// afterwards; anything else is kept, because the rest of it is not gofi's.
func Uninstall(projectRoot string) ([]Result, error) {
	dir, err := Dir(projectRoot)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, hook := range Managed {
		path := filepath.Join(dir, hook)
		old, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		next := cutBlock(string(old))
		if next == string(old) {
			continue
		}
		res := Result{Hook: hook, Path: path, Action: Removed}
		if isBare(next) {
			if err := os.Remove(path); err != nil {
				return results, err
			}
			results = append(results, res)
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0o755); err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// Installed lists the managed hooks that currently carry a gofi block.
func Installed(projectRoot string) []string {
	dir, err := Dir(projectRoot)
	if err != nil {
		return nil
	}
	var found []string
	for _, hook := range Managed {
		b, err := os.ReadFile(filepath.Join(dir, hook))
		if err != nil {
			continue
		}
		if strings.Contains(string(b), Begin) {
			found = append(found, hook)
		}
	}
	return found
}

// replaceBlock swaps the gofi block of a script, appending it when the script
// has none yet.
func replaceBlock(script, block string) string {
	from := strings.Index(script, Begin)
	to := strings.Index(script, End)
	if from < 0 || to < from {
		if !strings.HasSuffix(script, "\n") {
			script += "\n"
		}
		return script + block + "\n"
	}
	return script[:from] + block + script[to+len(End):]
}

// cutBlock removes the gofi block and the blank line it leaves behind.
func cutBlock(script string) string {
	from := strings.Index(script, Begin)
	to := strings.Index(script, End)
	if from < 0 || to < from {
		return script
	}
	rest := strings.TrimPrefix(script[to+len(End):], "\n")
	return script[:from] + rest
}

// isBare reports whether what is left of a script does nothing: blank lines,
// comments and a shebang, but no command.
func isBare(script string) bool {
	for line := range strings.Lines(script) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return false
	}
	return true
}
