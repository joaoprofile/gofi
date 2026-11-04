package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// historyKeep is how many versions the head of a context's memory keeps in
// its ## Histórico de versões; the older ones move to the context's
// history.md. Every role reads the head: a changelog that only grows is paid
// by every turn.
const historyKeep = 10

// stateLimit is the size past which the ## Estado atual is said to need
// consolidating: the state as it is today, not the diary of how it got there.
const stateLimit = 12 * 1024

const (
	historyHeading = "## Histórico de versões"
	stateHeading   = "## Estado atual"
)

// overflowHistory takes the versions past historyKeep out of a memory's head
// and returns the head left and the rows taken, newest first. A head without
// the table, or with no more rows than it keeps, is left as it is.
func overflowHistory(doc, ctx string) (string, []string) {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == historyHeading {
			start = i
			break
		}
	}
	if start < 0 {
		return doc, nil
	}
	var header, rows []int
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "## ") {
			end = i
			break
		}
		if !strings.HasPrefix(l, "|") {
			continue
		}
		if len(header) < 2 {
			header = append(header, i)
		} else {
			rows = append(rows, i)
		}
	}
	if len(rows) <= historyKeep {
		return doc, nil
	}
	moved := map[int]bool{}
	var taken []string
	for _, i := range rows[historyKeep:] {
		moved[i] = true
		taken = append(taken, lines[i])
	}
	pointer := fmt.Sprintf("Versões anteriores: `%s`.", layout.Contexts().Path(ctx, "history.md"))
	var out []string
	hasPointer := false
	for i, l := range lines {
		if moved[i] {
			continue
		}
		if i > start && i < end && strings.TrimSpace(l) == pointer {
			hasPointer = true
		}
		out = append(out, l)
		if i == rows[historyKeep-1] && !hasPointer && !strings.Contains(strings.Join(lines[start:end], "\n"), pointer) {
			out = append(out, "", pointer)
		}
	}
	return strings.Join(out, "\n"), taken
}

// appendHistory puts rows taken from the head on top of the context's
// history.md table: they are newer than the rows already there.
func appendHistory(root, ctx string, rows []string) error {
	path := layout.Contexts().Abs(root, ctx, "history.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	head := []string{"# " + ctx + " — versões anteriores", "", "| Versão | Data | Mudança |", "|--------|------|---------|"}
	var kept []string
	if len(old) > 0 {
		lines := strings.Split(strings.TrimRight(string(old), "\n"), "\n")
		seenTable := 0
		for _, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "|") {
				seenTable++
				if seenTable <= 2 {
					continue
				}
				kept = append(kept, l)
			}
		}
	}
	body := strings.Join(append(append(head, rows...), kept...), "\n") + "\n"
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// stateSize is the size of a memory's ## Estado atual.
func stateSize(doc string) int {
	_, rest, ok := strings.Cut(doc, stateHeading)
	if !ok {
		return 0
	}
	if i := strings.Index(rest, "\n## "); i >= 0 {
		rest = rest[:i]
	}
	return len(rest)
}

// memoryNote is what a write of a memory says beyond "written": the state
// grown past what every turn should read.
func memoryNote(doc string) string {
	if n := stateSize(doc); n > stateLimit {
		return i18n.T("memory.state_too_big", n/1024, stateLimit/1024)
	}
	return ""
}
