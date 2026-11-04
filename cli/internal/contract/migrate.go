package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Migrated is what migrating a context from format 2 did (0003, S10).
type Migrated struct {
	Context string
	Spec    string            // the living spec written
	Archive string            // where the old documents went
	Renamed map[string]string // old rule (file#RN-NN) → new id, when ids had to be renumbered
	Pending int               // scenarios left for a person to write
}

var (
	reRuleHeading = regexp.MustCompile(`^###\s+(RN-\d+)\s*(?:[—–-]\s*)?(.*)$`)
	// The how a format-2 rule carried in its body: it is the code's now.
	reRuleHow = regexp.MustCompile(`^\*\*(Implementação|Erro|Implementation|Error):\*\*`)
)

// Migrate turns the format-2 specs of a context into its living spec: the
// overview (§1) becomes Propósito, each RN-NN a REQ-NN with its text — the
// same number when the context had one spec, renumbered in order when it had
// several — and a scenario marked [PRECISA ESCLARECER] for a person to write,
// since a format-2 rule carried none. The rest of the old documents is the how,
// already in the code: they move whole to changes/archive/<date>-baseline-<ctx>/
// as the record. The living spec is born proposto; approving it is the
// person's, once the scenarios are written.
//
// Nothing is written when the context has no format-2 spec or already has a
// living spec.
func Migrate(root string, all []Doc, ctx, date string) (Migrated, error) {
	var old []Doc
	for _, d := range all {
		if d.Kind == KindSpec && d.Change == "" && d.Context == ctx && d.Format < FormatSplit {
			old = append(old, d)
		}
	}
	if len(old) == 0 {
		return Migrated{}, fmt.Errorf("%s não tem spec no formato 2", ctx)
	}
	target := layout.Specs().Path(ctx, "spec.md")
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err == nil {
		return Migrated{}, fmt.Errorf("%s já existe: o contexto já tem spec viva", target)
	}

	res := Migrated{Context: ctx, Spec: target, Renamed: map[string]string{},
		// The old documents are the context's history: they go to its own
		// archive, beside the living spec they become.
		Archive: layout.Specs().Path(ctx, ChangesDir, ArchiveDir, date+"-baseline-"+ctx)}
	var purpose, reqs []string
	next := 1
	for _, d := range old {
		purpose = append(purpose, trimBlank(numberedSection(d.Body, "1"))...)
		for _, r := range rulesOf(d.Body) {
			id := r.id
			if len(old) > 1 {
				id = fmt.Sprintf("REQ-%02d", next)
				res.Renamed[d.Path+"#"+r.id] = id
				next++
			} else {
				id = "REQ-" + strings.TrimPrefix(r.id, "RN-")
			}
			reqs = append(reqs, "", "### Requisito: "+id+" — "+r.title)
			reqs = append(reqs, r.text...)
			reqs = append(reqs, "",
				"#### Cenário: "+id+".1 — "+r.title,
				"- DADO [PRECISA ESCLARECER: cenário de "+id+" — a regra antiga não tinha critério verificável]",
				"- QUANDO …",
				"- ENTÃO …")
			res.Pending++
		}
	}
	if len(trimBlank(purpose)) == 0 {
		purpose = []string{"[PRECISA ESCLARECER: o propósito de " + ctx + " — a spec antiga não tinha §1]"}
	}

	first := old[0].Front
	front := []string{"tipo: spec", "formato: 3", "contexto: " + ctx}
	for _, key := range []string{"versao", "prd", FieldDepends, "entidades", "operacoes", "marketplaces", "keywords"} {
		if v := strings.TrimSpace(first[key]); v != "" {
			front = append(front, key+": "+v)
		}
	}
	front = append(front, "status: proposto", "atualizado: "+date)
	body := append([]string{"# Spec — " + ctx, "", "## Propósito", ""}, trimBlank(purpose)...)
	body = append(body, "", "## Requisitos")
	body = append(body, reqs...)
	text := "---\n" + strings.Join(front, "\n") + "\n---\n" + strings.Join(body, "\n") + "\n"

	dest := filepath.Join(root, filepath.FromSlash(res.Archive))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return res, err
	}
	for _, d := range old {
		src := filepath.Join(root, filepath.FromSlash(d.Path))
		raw, err := os.ReadFile(src)
		if err != nil {
			return res, err
		}
		archivedText, err := setFields(string(raw), [][2]string{{"status", "arquivado"}})
		if err != nil {
			archivedText = string(raw)
		}
		if err := writeAtomic(filepath.Join(dest, filepath.Base(d.Path)), archivedText); err != nil {
			return res, err
		}
	}
	if err := writeAtomic(filepath.Join(root, filepath.FromSlash(target)), text); err != nil {
		return res, err
	}
	for _, d := range old {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(d.Path))); err != nil {
			return res, err
		}
	}
	// The context's memory points at the spec it has now.
	mem := layout.Contexts().Abs(root, ctx+".md")
	if raw, err := os.ReadFile(mem); err == nil {
		if out, err := setFields(string(raw), [][2]string{{"spec", target}}); err == nil {
			_ = writeAtomic(mem, out)
		}
	}
	return res, nil
}

type oldRule struct {
	id, title string
	text      []string
}

// rulesOf reads the RN-NN blocks of a format-2 spec's §5, without the
// implementation notes they carried.
func rulesOf(body []string) []oldRule {
	var out []oldRule
	var cur *oldRule
	for _, ln := range numberedSection(body, "5") {
		if m := reRuleHeading.FindStringSubmatch(ln); m != nil {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &oldRule{id: m[1], title: strings.TrimSpace(m[2])}
			continue
		}
		if strings.HasPrefix(ln, "### ") {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}
		t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), ">"))
		t = strings.TrimSpace(strings.TrimSuffix(t, "  "))
		if t == "" || reRuleHow.MatchString(t) {
			continue
		}
		cur.text = append(cur.text, t)
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// numberedSection is the body of a format-2 "## N." section, up to the next
// ## heading.
func numberedSection(body []string, num string) []string {
	var out []string
	in := false
	for _, ln := range body {
		if m := reSection.FindStringSubmatch(ln); m != nil {
			in = m[1] == num
			continue
		}
		if reHeading2.MatchString(ln) {
			in = false
			continue
		}
		if in {
			out = append(out, ln)
		}
	}
	return out
}
