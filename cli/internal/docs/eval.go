package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoldenFile is where a project keeps the questions its corpus must answer.
const GoldenFile = ".claude/eval/golden.md"

// Question is one entry of the golden set: what someone asks, and where the
// answer actually is.
type Question struct {
	ID       string
	Ask      string
	Document string
	Section  string
}

// EvalResult is how well the index answers the golden set.
type EvalResult struct {
	Total                     int
	Recall1, Recall3, Recall5 int
	MRR                       float64
	AvgTokens, WorstTokens    int
	Misses                    []Question
}

// ReadGolden loads the golden set from its markdown table.
func ReadGolden(root string) ([]Question, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GoldenFile)))
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

// Evaluate measures the index against the golden set.
//
// The score is lexical, and it is honest about only one thing: whether the
// index can carry a question to the right document. Read it as the index's
// discriminating power — the variable a change to the corpus actually moves —
// and always as a comparison between two runs, never as an absolute claim about
// how often an agent succeeds.
func Evaluate(root string, idx *Index, s *Searcher) (*EvalResult, error) {
	questions, err := ReadGolden(root)
	if err != nil {
		return nil, err
	}
	byPath := map[string]Doc{}
	for _, d := range idx.Docs {
		byPath[d.Path] = d
	}
	res := &EvalResult{Total: len(questions)}
	sum := 0
	for _, q := range questions {
		hits := s.Search(q.Ask, 5)
		rank := 0
		var matched *Result
		for i := range hits {
			if hits[i].Doc.Path == q.Document {
				rank = i + 1
				matched = &hits[i]
				break
			}
		}
		switch {
		case rank == 1:
			res.Recall1++
			res.Recall3++
			res.Recall5++
		case rank > 0 && rank <= 3:
			res.Recall3++
			res.Recall5++
		case rank > 0:
			res.Recall5++
		}
		if rank > 0 {
			res.MRR += 1 / float64(rank)
		} else {
			res.Misses = append(res.Misses, q)
		}
		cost := 5 * 60 // what the tool prints back
		if matched != nil && matched.Section != nil {
			cost += sectionTokens(root, q.Document, matched.Section)
		}
		sum += cost
		if cost > res.WorstTokens {
			res.WorstTokens = cost
		}
	}
	if res.Total > 0 {
		res.MRR /= float64(res.Total)
		res.AvgTokens = sum / res.Total
	}
	return res, nil
}

// sectionTokens estimates what reading the pointed-at section costs.
func sectionTokens(root, doc string, sec *Section) int {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
	if err != nil {
		return 0
	}
	lines := strings.Split(string(raw), "\n")
	start, end := max(0, sec.Start-1), min(len(lines), sec.End)
	if start >= end {
		return 0
	}
	chars := 0
	for _, l := range lines[start:end] {
		chars += len(l) + 1
	}
	return chars * 10 / 36
}

// Format renders a result for a terminal.
func (r *EvalResult) Format() string {
	pct := func(n int) float64 {
		if r.Total == 0 {
			return 0
		}
		return float64(n) / float64(r.Total) * 100
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %d perguntas\n\n", r.Total)
	fmt.Fprintf(&b, "  recall@1   %5.1f%%   (%d/%d)\n", pct(r.Recall1), r.Recall1, r.Total)
	fmt.Fprintf(&b, "  recall@3   %5.1f%%   (%d/%d)\n", pct(r.Recall3), r.Recall3, r.Total)
	fmt.Fprintf(&b, "  recall@5   %5.1f%%   (%d/%d)\n", pct(r.Recall5), r.Recall5, r.Total)
	fmt.Fprintf(&b, "  MRR        %5.3f\n\n", r.MRR)
	fmt.Fprintf(&b, "  custo médio até a resposta: %d tokens\n", r.AvgTokens)
	fmt.Fprintf(&b, "  pior caso                 : %d tokens\n", r.WorstTokens)
	if len(r.Misses) > 0 {
		fmt.Fprintf(&b, "\n  %d sem resposta no top-5:\n", len(r.Misses))
		for _, m := range r.Misses {
			fmt.Fprintf(&b, "       %s  %s\n", m.ID, m.Ask)
		}
		b.WriteString("\n  Uma pergunta que falha costuma ser vocabulário, não documento faltando:\n")
		b.WriteString("  veja se o par que faltou pertence a .claude/lexicon/sinonimos.md.\n")
	}
	return b.String()
}
