package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// FormatVersion is the corpus format this CLI writes.
//
// Stamped on every document so migration is idempotent and resumable: a step
// processes only what is behind, and running it twice is a no-op. Without it a
// migration has to guess what it already did.
const FormatVersion = 2

const (
	maxKeywords = 12
	maxEntities = 10
	minDiagram  = 6
)

var (
	reFieldLine = regexp.MustCompile(`^([a-zA-Z_]+):`)
	rePlantUML  = regexp.MustCompile(`^(\s*)` + "```" + `plantuml\s*$`)
	reFenceEnd  = regexp.MustCompile("^\\s*```\\s*$")
	reTitle     = regexp.MustCompile(`^\s*title\s+`)
)

// MigrateResult is what a step did, or would do without Apply.
type MigrateResult struct {
	Step    string
	Changed int
	Skipped int
	Notes   []string
	Samples []string
}

// Migrator runs the one-time steps that bring a corpus to the current format.
type Migrator struct {
	Root  string
	Apply bool
}

// Stamp writes the format marker. Everything else keys off it.
func (m *Migrator) Stamp() (*MigrateResult, error) {
	res := &MigrateResult{Step: "stamp"}
	for _, corpus := range Corpora {
		err := walkCorpus(m.Root, corpus, func(path string, fm Frontmatter, body []string) {
			if _, ok := fm["formato"]; ok {
				res.Skipped++
				return
			}
			lines, _ := frontmatterLines(path)
			line := fmt.Sprintf("formato: %d", FormatVersion)
			out := insertAfter(lines, "tipo", line)
			if m.Apply {
				_ = writeDoc(path, out, body)
			}
			res.Changed++
		})
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// Lexicon derives the controlled vocabulary from the project itself: entities
// from the schema, operations from the keywords already shared between
// contexts. Nothing is invented.
func (m *Migrator) Lexicon() (*MigrateResult, error) {
	res := &MigrateResult{Step: "lexicon"}

	tables := SchemaTables(m.Root)
	if len(tables) == 0 {
		res.Notes = append(res.Notes,
			"nenhuma migration encontrada — 'entidades' fica vazio, e com ele o elo entre documento e código")
	}

	contextsOf := map[string]map[string]bool{}
	promoted := map[string]bool{}
	for _, corpus := range []string{"specs", "prd"} {
		_ = walkCorpus(m.Root, corpus, func(_ string, fm Frontmatter, _ []string) {
			ctx := fm.Get("contexto")
			for _, k := range fm.List("keywords") {
				if contextsOf[k] == nil {
					contextsOf[k] = map[string]bool{}
				}
				contextsOf[k][ctx] = true
			}
			// A term already promoted to a facet has left keywords: ignoring it
			// would shrink the lexicon on a second run and invalidate the facets
			// of documents already migrated.
			for _, o := range fm.List(LexOperations) {
				promoted[o] = true
			}
		})
	}

	isTable := map[string]bool{}
	for _, t := range tables {
		isTable[t] = true
	}
	markets := ReadLexicon(m.Root, LexMarketplaces)

	ops := map[string]bool{}
	for t := range promoted {
		ops[t] = true
	}
	for _, t := range ReadLexicon(m.Root, LexOperations).Terms {
		ops[t] = true
	}
	for term, ctxs := range contextsOf {
		if len(ctxs) >= 2 {
			ops[term] = true
		}
	}
	var operations []string
	for t := range ops {
		if !isTable[strings.ReplaceAll(t, "-", "_")] && !markets.Has(t) {
			operations = append(operations, t)
		}
	}
	sort.Strings(operations)

	before := len(ReadLexicon(m.Root, LexEntities).Terms) +
		len(ReadLexicon(m.Root, LexOperations).Terms)

	if m.Apply {
		if len(tables) > 0 {
			if err := WriteLexicon(m.Root, LexEntities, "Entidades",
				"Tabelas reais do schema. **Derivado** — regenere com `gofi docs migrate lexicon`. "+
					"É o léxico que liga documento a símbolo de código.", tables); err != nil {
				return res, err
			}
		}
		if err := WriteLexicon(m.Root, LexOperations, "Operações",
			"Processos e conceitos compartilhados por ≥2 contextos. Derivado das keywords "+
				"existentes e depois curado. Termo novo entra **aqui** antes de entrar num "+
				"documento — é o que impede a volta da nuvem de tags.", operations); err != nil {
			return res, err
		}
		for _, seed := range []struct{ name, title, note, col string }{
			{LexMarketplaces, "Marketplaces", "Canais de venda. O canônico é o que vai no " +
				"frontmatter; os apelidos existem só para detecção — sem eles, buscar por um " +
				"não acha o outro.", "Apelidos"},
			{LexSynonyms, "Sinônimos", "Ponte entre a língua da pergunta e a do identificador. " +
				"**Cresce a cada busca que falha:** registre aqui o par que faltou.", "Equivale a"},
		} {
			path := filepath.Join(m.Root, LexiconDir, seed.name+".md")
			if _, err := os.Stat(path); err == nil {
				continue // hand-kept: never overwrite
			}
			body := fmt.Sprintf("# %s\n\n> %s\n\n| Termo | %s |\n|---|---|\n",
				seed.title, seed.note, seed.col)
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return res, err
			}
		}
	}
	if after := len(tables) + len(operations); after != before {
		res.Changed = after - before
	} else {
		res.Skipped = after
	}
	res.Notes = append(res.Notes, fmt.Sprintf("%d entidades (schema) · %d operações · "+
		"marketplaces e sinônimos são curadoria manual", len(tables), len(operations)))
	return res, nil
}

// Facets replaces the free tag cloud with the controlled axes.
func (m *Migrator) Facets() (*MigrateResult, error) {
	res := &MigrateResult{Step: "facets"}
	entities := ReadLexicon(m.Root, LexEntities)
	operations := ReadLexicon(m.Root, LexOperations)
	markets := ReadLexicon(m.Root, LexMarketplaces)

	for _, corpus := range []string{"specs", "prd"} {
		err := walkCorpus(m.Root, corpus, func(path string, fm Frontmatter, body []string) {
			text := strings.Join(body, "\n")
			keywords := fm.List("keywords")

			ent := detectEntities(text, entities.Terms)
			mkt := detectMarketplaces(text, markets)
			// Operations come from the keywords AND from the operacoes field a
			// previous run already wrote. Reading only keywords would drop the
			// facet on a second pass, since the terms have left keywords by
			// then — the step has to be idempotent to be safe to re-run.
			seenOps := map[string]bool{}
			var ops []string
			for _, k := range append(append([]string{}, keywords...), fm.List(LexOperations)...) {
				if operations.Has(k) && !seenOps[k] {
					seenOps[k] = true
					ops = append(ops, k)
				}
			}
			sort.Strings(ops)
			covered := map[string]bool{}
			for _, o := range ops {
				covered[o] = true
			}
			for _, e := range ent {
				covered[strings.ReplaceAll(e, "_", "-")] = true
			}
			for _, k := range mkt {
				covered[k] = true
			}
			var rest []string
			for _, k := range keywords {
				if !covered[k] && len(rest) < maxKeywords {
					rest = append(rest, k)
				}
			}

			lines, _ := frontmatterLines(path)
			out := replaceKeywords(lines, ent, mkt, ops, rest)
			// Counting a rewrite that produces identical bytes as a change
			// makes an idempotent step look like it did work every run, which
			// is exactly the signal someone re-running it needs to trust.
			if sameLines(lines, out) {
				res.Skipped++
				return
			}
			if m.Apply {
				_ = writeDoc(path, out, body)
			} else if len(res.Samples) < 3 {
				res.Samples = append(res.Samples, fmt.Sprintf(
					"%s\n    entidades   : %s\n    marketplaces: %s\n    operacoes   : %s\n    keywords    : %s",
					relPath(m.Root, path), orDash(ent), orDash(mkt), orDash(ops), orDash(rest)))
			}
			res.Changed++
		})
		if err != nil {
			return res, err
		}
	}
	if len(entities.Terms) == 0 {
		res.Notes = append(res.Notes, "léxico de entidades vazio — rode 'migrate lexicon' antes")
	}
	return res, nil
}

// detectEntities only counts an identifier that is marked as one.
//
// A bare mention in prose is a false positive: product, order and account are
// ordinary words in technical writing. The corpus already marks identifiers
// with backticks, and DDL is stronger evidence still. Ranked by ownership: the
// document that CREATEs the table comes before one that merely names it.
func detectEntities(text string, tables []string) []string {
	weight := map[string]int{}
	for _, t := range tables {
		ticks := strings.Count(text, "`"+t+"`")
		defines := regexp.MustCompile(`(?i)CREATE TABLE (?:IF NOT EXISTS )?"?` +
			regexp.QuoteMeta(t) + `(?:[^a-z0-9_]|$)`).MatchString(text)
		if defines || ticks > 0 {
			w := ticks
			if defines {
				w += 1000
			}
			weight[t] = w
		}
	}
	found := make([]string, 0, len(weight))
	for t := range weight {
		found = append(found, t)
	}
	sort.Slice(found, func(i, j int) bool {
		if weight[found[i]] != weight[found[j]] {
			return weight[found[i]] > weight[found[j]]
		}
		return found[i] < found[j]
	})
	if len(found) > maxEntities {
		found = found[:maxEntities]
	}
	sort.Strings(found)
	return found
}

func detectMarketplaces(text string, lex Lexicon) []string {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	for alias, canon := range lex.Alias {
		re := regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])` + regexp.QuoteMeta(strings.ToLower(alias)) +
			`(?:[^a-z0-9_]|$)`)
		if re.MatchString(lower) {
			seen[canon] = true
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Diagrams lifts PlantUML out of the specs.
//
// A diagram is image source, not prose: inline it inflates the section an agent
// reads to answer a question, while the text beside it already says what the
// diagram shows. Outside, it stays versioned and rendered, and is opened only
// when someone wants the diagram itself.
func (m *Migrator) Diagrams() (*MigrateResult, error) {
	res := &MigrateResult{Step: "diagrams"}
	err := walkCorpus(m.Root, "specs", func(path string, _ Frontmatter, _ []string) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		lines := strings.Split(string(raw), "\n")
		var out []string
		count, lifted := 0, 0
		stem := strings.TrimSuffix(filepath.Base(path), ".md")
		for i := 0; i < len(lines); {
			open := rePlantUML.FindStringSubmatch(lines[i])
			if open == nil {
				out = append(out, lines[i])
				i++
				continue
			}
			j := i + 1
			var block []string
			for j < len(lines) && !reFenceEnd.MatchString(lines[j]) {
				block = append(block, lines[j])
				j++
			}
			if len(block) < minDiagram {
				out = append(out, lines[i:min(j+1, len(lines))]...)
				i = j + 1
				continue
			}
			count++
			name := fmt.Sprintf("%s-%d.puml", stem, count)
			if m.Apply {
				dir := filepath.Join(filepath.Dir(path), "diagrams")
				_ = os.MkdirAll(dir, 0o755)
				_ = os.WriteFile(filepath.Join(dir, name),
					[]byte(strings.Join(block, "\n")+"\n"), 0o644)
			}
			label := ""
			for _, b := range block {
				if reTitle.MatchString(b) {
					label = " — " + strings.TrimSpace(reTitle.ReplaceAllString(b, ""))
					break
				}
			}
			out = append(out, fmt.Sprintf(
				"%s> **Diagrama**%s: [`diagrams/%s`](diagrams/%s) (%d linhas PlantUML)",
				open[1], label, name, name, len(block)))
			lifted += len(block)
			i = j + 1
		}
		if count > 0 {
			res.Changed += count
			res.Skipped += lifted
			if m.Apply {
				_ = os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
			}
		}
	})
	return res, err
}

// --- frontmatter rewriting -------------------------------------------------

func frontmatterLines(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[1:i], nil
		}
	}
	return nil, nil
}

func writeDoc(path string, frontmatter, body []string) error {
	all := append([]string{"---"}, frontmatter...)
	all = append(all, "---")
	all = append(all, body...)
	text := strings.Join(all, "\n")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func insertAfter(lines []string, field, line string) []string {
	for i, l := range lines {
		if m := reFieldLine.FindStringSubmatch(l); m != nil && m[1] == field {
			out := append([]string{}, lines[:i+1]...)
			out = append(out, line)
			return append(out, lines[i+1:]...)
		}
	}
	return append([]string{line}, lines...)
}

// replaceKeywords swaps the keywords line for the facet lines, in place, so the
// rest of the frontmatter keeps the order its author gave it.
func replaceKeywords(lines []string, ent, mkt, ops, rest []string) []string {
	facets := func() []string {
		var out []string
		for _, f := range []struct {
			name  string
			terms []string
		}{{LexEntities, ent}, {LexMarketplaces, mkt}, {LexOperations, ops}} {
			if len(f.terms) > 0 {
				out = append(out, fmt.Sprintf("%s: [%s]", f.name, strings.Join(f.terms, ", ")))
			}
		}
		return append(out, fmt.Sprintf("keywords: [%s]", strings.Join(rest, ", ")))
	}
	var out []string
	replaced := false
	for _, l := range lines {
		if m := reFieldLine.FindStringSubmatch(l); m != nil && m[1] == "keywords" {
			out = append(out, facets()...)
			replaced = true
			continue
		}
		// Drop any facet line from a previous run so this is idempotent.
		if m := reFieldLine.FindStringSubmatch(l); m != nil {
			switch m[1] {
			case LexEntities, LexMarketplaces, LexOperations:
				continue
			}
		}
		out = append(out, l)
	}
	if !replaced {
		out = append(out, facets()...)
	}
	return out
}

func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func orDash(in []string) string {
	if len(in) == 0 {
		return "—"
	}
	return strings.Join(in, ", ")
}
