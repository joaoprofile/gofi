package chat

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// localCommands are the chat's own commands, offered by tab completion ahead
// of the project's skills.
var localCommands = []string{"/help", "/clear", "/model", "/resume", "/exit"}

// complete finishes the word under the cursor — the last one typed: a /command
// or /skill at the start of the line, or an @path anywhere. It returns the new
// input and, when the word is still ambiguous, the candidates to show.
func (m model) complete(input string) (string, []string) {
	start := strings.LastIndexAny(input, " \t") + 1
	word := input[start:]
	var candidates []string
	switch {
	case strings.HasPrefix(word, "/") && start == 0:
		for _, c := range append(append([]string{}, localCommands...), prefixed("/", m.skills)...) {
			if strings.HasPrefix(c, word) {
				candidates = append(candidates, c)
			}
		}
	case strings.HasPrefix(word, "@"):
		candidates = pathCandidates(m.opts.Root, word[1:])
	default:
		return input, nil
	}
	candidates = unique(candidates)
	switch len(candidates) {
	case 0:
		return input, nil
	case 1:
		done := candidates[0]
		if !strings.HasSuffix(done, "/") {
			done += " "
		}
		return input[:start] + done, nil
	}
	return input[:start] + commonPrefix(candidates), candidates
}

// pathCandidates lists the entries of the folder partial points into, under
// root, whose names start with what follows the last slash. Folders end in
// "/", so the next tab goes into them. Hidden entries only when asked for.
func pathCandidates(root, partial string) []string {
	dir, prefix := filepath.Split(filepath.FromSlash(partial))
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".")) {
			continue
		}
		p := filepath.ToSlash(filepath.Join(dir, name))
		if e.IsDir() {
			p += "/"
		}
		out = append(out, "@"+p)
	}
	sort.Strings(out)
	return out
}

func prefixed(p string, items []string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if !strings.HasPrefix(it, p) {
			it = p + it
		}
		out = append(out, it)
	}
	return out
}

func unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}

func commonPrefix(items []string) string {
	p := items[0]
	for _, it := range items[1:] {
		for !strings.HasPrefix(it, p) {
			p = p[:len(p)-1]
		}
	}
	return p
}
