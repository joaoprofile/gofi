package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The discovery's sensor. A PRD is what the specification is written from: it
// is ready for that when nothing in it still asks a question, the template was
// filled in, and it says what a requirement can be written from — the
// problem, how success is measured, who it is for, what is in and what is
// out, the journeys. The discovery's guides — the role, its playbook — say how
// to get there; this says, with no model asked, whether it got there.
//
// It reads a PRD of the current format only: an older one was written under
// other rules, and is brought over by its migration.

// prdSections are the sections the PRD template has, by number.
var prdSections = []struct{ num, name string }{
	{"1", "Problema"},
	{"2", "Objetivo e métricas de sucesso"},
	{"3", "Personas"},
	{"4", "Escopo"},
	{"5", "Jornadas"},
	{"6", "Restrições, premissas e riscos"},
	{"7", "Dúvidas em aberto"},
}

var (
	reDoubt       = regexp.MustCompile(`\[PRECISA ESCLARECER[^\]]*\]?`)
	rePRDToken    = regexp.MustCompile(`\{[^{}\n]+\}`)
	reRequirement = regexp.MustCompile(`^#{2,5}\s+(Requisito|Cen[áa]rio|Requirement|Scenario):`)
	reStep        = regexp.MustCompile(`^\s*\d+[.)]\s+\S`)
	reItem        = regexp.MustCompile(`^\s*[-*]\s+\S`)
	reSystemMust  = regexp.MustCompile(`(?i)\bo sistema (deve|n[ãa]o deve)\b`)
)

// PRDPath is where the PRD of a context is, relative to the project root.
func PRDPath(context string) string {
	return layout.PRD().Path(context, "prd-"+context+".md")
}

// PRDs are the PRDs a project has, by path relative to its root.
func PRDs(root string) []string {
	found, _ := filepath.Glob(layout.PRD().Abs(root, "*", "prd-*.md"))
	var out []string
	for _, f := range found {
		if rel, err := filepath.Rel(root, f); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out
}

// What CheckPRD could read of a PRD.
const (
	PRDMissing = iota // no such PRD
	PRDOlder          // a PRD of an older format: only its header is checked
	PRDCurrent        // a PRD of the current format: what it says is judged
)

// CheckPRD reports what stands between a PRD and the specification: what it
// says, and the corpus findings — fields, lexicon — `gofi index check` reports
// for the same file, so one command closes a PRD. An error asks for a
// correction before the contract is written from it; a warning is worth
// reading. A PRD of an older format was written under other sections: its
// header is checked and nothing is said of what it says.
func CheckPRD(root, rel string) (findings []docs.Finding, state int) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, PRDMissing
	}
	fm, body := docs.Parse(string(raw))
	if value(fm, "tipo") != "prd" {
		return nil, PRDMissing
	}
	f := finder{doc: Doc{Path: rel, Front: fm, Body: body, Offset: bodyLine(string(raw), body)}}
	corpus := func() {
		for _, c := range docs.Validate(root) {
			if c.Path == rel {
				f.out = append(f.out, c)
			}
		}
	}
	if formatOf(fm) < FormatSplit {
		corpus()
		return f.out, PRDOlder
	}

	// What the template leaves to be filled in, as the template writes it: a
	// brace of the PRD's own is not one of these.
	tokens := map[string]bool{}
	if tpl, err := os.ReadFile(layout.Templates().Abs(root, "prd-template.md")); err == nil {
		for _, t := range rePRDToken.FindAllString(string(tpl), -1) {
			tokens[t] = true
		}
	}

	sections := map[string][]int{} // the lines of each section, by its number
	section, fenced := "", false
	for i, ln := range body {
		line := f.doc.Offset + i
		for _, t := range rePRDToken.FindAllString(ln, -1) {
			if tokens[t] {
				f.add(line, true, "trecho do template não preenchido: %s", t)
				break
			}
		}
		if docs.IsFence(ln) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := reSection.FindStringSubmatch(ln); m != nil {
			section = m[1]
			sections[section] = []int{}
			continue
		}
		if m := reDoubt.FindString(ln); m != "" && !tokens[rePRDToken.FindString(m)] {
			f.add(line, true, "dúvida em aberto — responda antes da especificação: %s", clip(m, 110))
		}
		if m := reOpenMarker.FindString(ln); m != "" {
			f.add(line, true, "marcador em aberto %q: decida antes da especificação", m)
		}
		if reRequirement.MatchString(ln) {
			f.add(line, true, "requisito ou cenário no PRD: eles nascem no delta da mudança, com /gofi-spec")
		} else if reSystemMust.MatchString(ln) && !strings.HasPrefix(strings.TrimSpace(ln), ">") {
			f.add(line, false, "\"o sistema deve…\" é frase de requisito: o PRD diz por quê e para quem")
		}
		if section != "" {
			sections[section] = append(sections[section], i)
		}
	}

	for _, s := range prdSections {
		if _, ok := sections[s.num]; !ok {
			f.add(0, true, "falta a seção %s (%s)", s.num, s.name)
		}
	}
	// said reports whether a section says something of its own: a line that
	// is not a heading, the template's hint, or a table's frame.
	said := func(num string, keep func(string) bool) bool {
		for _, i := range sections[num] {
			ln := strings.TrimSpace(body[i])
			if ln == "" || strings.HasPrefix(ln, ">") || strings.HasPrefix(ln, "#") || rePRDToken.MatchString(ln) && tokens[rePRDToken.FindString(ln)] {
				continue
			}
			if keep == nil || keep(ln) {
				return true
			}
		}
		return false
	}
	need := func(num, what string, keep func(string) bool) {
		if _, ok := sections[num]; ok && !said(num, keep) {
			f.add(0, true, "§%s %s", num, what)
		}
	}
	need("1", "sem o problema: o que dói, para quem e a que custo", func(ln string) bool { return !strings.HasPrefix(ln, "|") })
	need("2", "sem métrica com hoje, meta e como medir", func(ln string) bool {
		c := cells(ln)
		return len(c) >= 4 && c[0] != "" && c[1] != "" && c[2] != "" && c[3] != ""
	})
	need("3", "sem nenhuma persona", func(ln string) bool { c := cells(ln); return len(c) >= 2 && c[0] != "" })
	need("5", "sem jornada com passos: é delas que nascem os requisitos", reStep.MatchString)
	if _, ok := sections["6"]; ok && !said("6", func(ln string) bool { c := cells(ln); return len(c) >= 2 && c[1] != "" }) {
		f.add(0, false, "§6 sem restrição, premissa ou risco descrito: o que se assume fica implícito")
	}
	if lines, ok := sections["4"]; ok {
		// In and out are both said: what is left out is half of the scope.
		in := map[string]bool{}
		part := ""
		for _, i := range lines {
			ln := strings.TrimSpace(body[i])
			switch {
			case strings.HasPrefix(ln, "### "):
				part = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(ln, "### ")))
			case reItem.MatchString(ln) && !(rePRDToken.MatchString(ln) && tokens[rePRDToken.FindString(ln)]):
				in[part] = true
			}
		}
		for _, p := range []string{"dentro", "fora"} {
			if !in[p] {
				f.add(0, true, "§4 sem nada em \"%s\": o escopo diz o que entra e o que fica de fora", strings.ToUpper(p[:1])+p[1:])
			}
		}
	}
	corpus()
	return f.out, PRDCurrent
}

// cells are the cells of a table's data row; nil for its header's rule, for
// the header itself as the template writes it, and for a line that is no row.
func cells(ln string) []string {
	ln = strings.TrimSpace(ln)
	if !strings.HasPrefix(ln, "|") || strings.HasPrefix(strings.ReplaceAll(ln, " ", ""), "|-") {
		return nil
	}
	parts := strings.Split(strings.Trim(ln, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch parts[0] {
	case "Métrica", "Persona", "Tipo":
		return nil // the template's header row
	}
	return parts
}

// clip shortens a text to at most n runes.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// PRDSummary says in a line what CheckPRD found: how much asks for a
// correction and how much is only worth reading.
func PRDSummary(findings []docs.Finding) (errs, warns int) {
	errs = Errors(findings)
	return errs, len(findings) - errs
}

// PRDLines are the findings as lines to show, at most limit of them: the
// corrections first, what a whole section lacks before what a line carries,
// and the pieces of the template left in as one line — a PRD that is still
// the template would otherwise say nothing else.
func PRDLines(findings []docs.Finding, limit int) []string {
	const leftover = "trecho do template não preenchido: "
	var left []string
	var rest []docs.Finding
	for _, f := range findings {
		if i := strings.Index(f.Message, leftover); i >= 0 {
			left = append(left, strings.TrimSuffix(f.Message[:i], ": ")+" "+f.Message[i+len(leftover):])
			continue
		}
		rest = append(rest, f)
	}
	rank := func(f docs.Finding) int {
		r := 0
		if !f.Error {
			r += 2
		}
		if strings.HasPrefix(f.Message, "L") {
			r++
		}
		return r
	}
	sort.SliceStable(rest, func(i, j int) bool { return rank(rest[i]) < rank(rest[j]) })
	var out []string
	if n := len(left); n > 0 {
		shown := left
		more := ""
		if n > 3 {
			shown, more = left[:3], "…"
		}
		out = append(out, fmt.Sprintf("✗ %d trecho(s) do template sem preencher: %s%s", n, strings.Join(shown, ", "), more))
	}
	for _, f := range rest {
		mark := "·"
		if f.Error {
			mark = "✗"
		}
		out = append(out, mark+" "+f.Message)
	}
	if limit > 0 && len(out) > limit {
		out = append(out[:limit], fmt.Sprintf("… +%d", len(out)-limit))
	}
	return out
}
