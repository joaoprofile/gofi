package docs

import (
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"os"
	"path/filepath"
	"strings"
)

// GoldenFile is where a project keeps the questions its corpus must answer.
func GoldenFile() string { return layout.Eval().Path("golden.md") }

// Question is one entry of the golden set: what someone asks, and where the
// answer actually is.
type Question struct {
	ID       string
	Ask      string
	Document string
	Section  string
}

// ReadGolden loads the golden set from its markdown table.
func ReadGolden(root string) ([]Question, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GoldenFile())))
	if err != nil {
		return nil, err
	}
	var out []Question
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitRow(line)
		if len(cells) < 3 || isRuler(cells) || strings.EqualFold(cells[0], "#") {
			continue
		}
		q := Question{
			ID: cells[0], Ask: cells[1],
			Document: strings.Trim(cells[2], "`"),
		}
		if len(cells) > 3 {
			q.Section = cells[3]
		}
		if q.Ask != "" && q.Document != "" {
			out = append(out, q)
		}
	}
	return out, nil
}
