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

// requiredSectionsV2 are the sections a format-2 spec is worked from: the
// overview says what the context is, the operations what to implement, the
// rules what to test and audit against. Matched by number, as format 2 is
// read; format 3 drops section numbers as a contract (0003, S8).
var requiredSectionsV2 = []struct{ num, name string }{
	{"1", "Visão Geral"},
	{"4", "Operações"},
	{"5", "Regras de Negócio"},
}

var (
	reSection = regexp.MustCompile(`^##\s+(\d+(?:\.\d+)?)[.\s]`)
	reRule    = regexp.MustCompile(`^###\s+(RN-\d+)\b`)
	reFence   = regexp.MustCompile("^\\s*(```|~~~)")
	// A template placeholder is a braced phrase that starts in upper case:
	// {Contexto}, {Nome da Regra}. Lower-case braces are left alone, because
	// a route parameter ({id}) is written exactly like one.
	rePlaceholder = regexp.MustCompile(`\{[A-ZÀ-Ý][^{}\n]*\}`)
	reOpenMarker  = regexp.MustCompile(`\b(TODO|TBD|FIXME)\b|\?\?\?`)
)

// Check reports what stands between one document and the loop: what keeps it
// from being approved, and an approval that no longer holds. Errors block;
// warnings are worth reading and block nothing.
func Check(root string, d Doc, placeholders map[string]bool) []docs.Finding {
	return append(content(d, placeholders), sealFindings(d)...)
}

// content checks what a document says, by the rules of its kind and format.
func content(d Doc, placeholders map[string]bool) []docs.Finding {
	f := finder{doc: d}
	if len(d.Contexts) == 0 {
		f.add(0, true, "sem contexto no frontmatter")
	}
	f.leftovers(placeholders)
	if d.Kind == KindSpec && d.Format < FormatSplit {
		f.rulesV2()
	}
	return f.out
}

// finder collects the findings about one document.
type finder struct {
	doc Doc
	out []docs.Finding
}

func (f *finder) add(line int, isErr bool, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if line > 0 {
		msg = fmt.Sprintf("L%d: %s", line, msg)
	}
	f.out = append(f.out, docs.Finding{Path: f.doc.Path, Message: msg, Error: isErr})
}

// leftovers reports what a writer left in: a placeholder of a template, on
// any line — a heading or a code block can carry one as well as prose — and
// an open marker outside code.
func (f *finder) leftovers(placeholders map[string]bool) {
	fenced := false
	for i, ln := range f.doc.Body {
		line := f.doc.Offset + i
		for _, p := range rePlaceholder.FindAllString(ln, -1) {
			if placeholders[p] {
				f.add(line, true, "placeholder do template %s não preenchido", p)
			}
		}
		if reFence.MatchString(ln) {
			fenced = !fenced
			continue
		}
		if !fenced {
			if m := reOpenMarker.FindString(ln); m != "" {
				f.add(line, true, "marcador em aberto %q: decida antes de aprovar", m)
			}
		}
	}
}

// rulesV2 are the rules of a format-2 spec: numbered sections and RN-xx rules
// with a description.
func (f *finder) rulesV2() {
	sections := map[string]bool{}
	rules := map[string]int{}
	var ruleOrder []string
	inRules, fenced := false, false
	var rule string
	ruleText := map[string]bool{}
	for i, ln := range f.doc.Body {
		line := f.doc.Offset + i
		if reFence.MatchString(ln) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := reSection.FindStringSubmatch(ln); m != nil {
			sections[m[1]] = true
			inRules, rule = m[1] == "5", ""
			continue
		}
		if !inRules {
			continue
		}
		if m := reRule.FindStringSubmatch(ln); m != nil {
			rule = m[1]
			if first, dup := rules[rule]; dup {
				f.add(line, true, "%s repetida (já declarada na L%d)", rule, first)
			} else {
				rules[rule] = line
				ruleOrder = append(ruleOrder, rule)
			}
			continue
		}
		if strings.HasPrefix(ln, "### ") {
			rule = ""
		} else if rule != "" && strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(ln), ">")) != "" {
			ruleText[rule] = true
		}
	}
	for _, sec := range requiredSectionsV2 {
		if !sections[sec.num] {
			f.add(0, true, "falta a seção %s (%s)", sec.num, sec.name)
		}
	}
	if sections["5"] && len(rules) == 0 {
		f.add(0, false, "§5 sem nenhuma regra RN-xx: o loop não terá o que rastrear até o teste")
	}
	for _, r := range ruleOrder {
		if !ruleText[r] {
			f.add(rules[r], true, "%s sem descrição", r)
		}
	}
}

// Placeholders reads the placeholders of every template the project has —
// the corpus templates and each pack's — so the check knows a leftover from a
// phrase that merely sits in braces. With no template there is nothing to
// compare against, and ok is false.
func Placeholders(root string) (map[string]bool, bool) {
	var files []string
	for _, pattern := range []string{
		layout.Templates().Abs(root, "*.md"),
		layout.SDK().Abs(root, "*", "templates", "*.md"),
	} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	set := map[string]bool{}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, p := range rePlaceholder.FindAllString(string(b), -1) {
			set[p] = true
		}
	}
	return set, len(files) > 0
}

// CheckAll checks the documents picked out of the whole corpus (all): their
// content, their seal, the dependencies they declare, and the corpus findings
// — format, unknown fields, lexicon — `gofi index check` reports for the same
// files, so one command says everything that stands between them and the
// loop.
func CheckAll(root string, all, picked []Doc) []docs.Finding {
	placeholders, ok := Placeholders(root)
	var out []docs.Finding
	if !ok {
		out = append(out, docs.Finding{Path: layout.Templates().Path(),
			Message: "nenhum template: placeholders não verificados — rode `gofi update templates`"})
	}
	mine := map[string]bool{}
	for _, d := range picked {
		mine[d.Path] = true
		out = append(out, Check(root, d, placeholders)...)
	}
	out = append(out, depFindings(all, mine)...)
	for _, f := range docs.Validate(root) {
		if mine[f.Path] {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Errors counts the findings that block an approval.
func Errors(fs []docs.Finding) int {
	n := 0
	for _, f := range fs {
		if f.Error {
			n++
		}
	}
	return n
}
