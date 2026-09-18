package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MemoryLineCap is when a context memory file stops being a consolidated state
// and becomes an archive. Reported, never truncated: deciding what is still
// current takes domain judgement, and cutting by a ruler destroys knowledge.
const MemoryLineCap = 250

// Finding is one problem with the corpus. Errors fail the gate; warnings do not.
type Finding struct {
	Path    string
	Message string
	Error   bool
}

// Required frontmatter, by corpus. Context memory has no tipo: the folder says it.
var required = map[string][]string{
	"specs":                   {"tipo", "formato", "contexto", "versao", "status"},
	"prd":                     {"tipo", "formato", "contexto", "versao", "status"},
	".claude/memory/contexts": {"formato", "contexto", "versao", "status"},
}

// known is every field the format defines.
//
// spec/specs and prd/prds are BOTH here and are NOT drift: singular names the
// primary document, plural lists them all. They coexist in the same file, and
// "normalizing" one into the other overwrites the list.
var known = map[string]bool{
	"tipo": true, "formato": true, "contexto": true, "submodulo": true,
	"submodulo_de": true, "versao": true, "status": true,
	"prd": true, "prds": true, "spec": true, "specs": true,
	"entidades": true, "marketplaces": true, "operacoes": true, "keywords": true,
	"atualizado": true, "servicos": true, "diretorio": true,
	"versao_spec": true, "versao_prd": true, "feature": true,
}

// facetAxes are the fields whose terms must exist in the lexicon. This is what
// stops the vocabulary from drifting back into a tag cloud: without it, terms
// that appear in exactly one document accumulate until the index stops
// discriminating anything.
var facetAxes = []string{LexEntities, LexOperations, LexMarketplaces}

// Validate checks the corpus and reports what is wrong.
func Validate(root string) []Finding {
	lexicons := map[string]Lexicon{}
	for _, axis := range facetAxes {
		lexicons[axis] = ReadLexicon(root, axis)
	}
	var out []Finding
	add := func(path, msg string, isErr bool) {
		out = append(out, Finding{Path: path, Message: msg, Error: isErr})
	}

	corpora := make([]string, 0, len(required))
	for c := range required {
		corpora = append(corpora, c)
	}
	sort.Strings(corpora)

	for _, corpus := range corpora {
		_ = walkCorpus(root, corpus, func(path string, fm Frontmatter, body []string) {
			rel := relPath(root, path)
			for _, field := range required[corpus] {
				if _, ok := fm[field]; !ok {
					add(rel, fmt.Sprintf("falta o campo obrigatório %q", field), true)
				}
			}
			fields := make([]string, 0, len(fm))
			for f := range fm {
				fields = append(fields, f)
			}
			sort.Strings(fields)
			for _, f := range fields {
				if !known[f] {
					add(rel, fmt.Sprintf("campo desconhecido %q", f), true)
				}
			}
			for _, axis := range facetAxes {
				lex := lexicons[axis]
				if len(lex.Terms) == 0 {
					continue
				}
				for _, term := range fm.List(axis) {
					if !lex.Has(term) {
						add(rel, fmt.Sprintf("%s: %q fora do léxico", axis, term), true)
					}
				}
			}
			if corpus == ".claude/memory/contexts" {
				if n := len(body) + len(fm) + 2; n > MemoryLineCap {
					add(rel, fmt.Sprintf("%d linhas (teto %d) — transbordar o histórico "+
						"antigo para history.md", n, MemoryLineCap), false)
				}
			}
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Error != out[j].Error {
			return !out[i].Error // warnings first, errors last and closest to the eye
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// ValidateLexicon reports terms nobody uses, which is how a controlled
// vocabulary rots: entries accumulate and stop meaning anything.
func ValidateLexicon(root string) []string {
	used := map[string]bool{}
	for corpus := range required {
		_ = walkCorpus(root, corpus, func(_ string, fm Frontmatter, _ []string) {
			for _, axis := range facetAxes {
				for _, t := range fm.List(axis) {
					used[t] = true
				}
			}
		})
	}
	var unused []string
	// Entities come from the schema, so an unused one means a table nothing
	// documents — worth knowing, but not the same kind of rot.
	for _, axis := range []string{LexOperations, LexMarketplaces} {
		for _, t := range ReadLexicon(root, axis).Terms {
			if !used[t] {
				unused = append(unused, axis+": "+t)
			}
		}
	}
	sort.Strings(unused)
	return unused
}

// UndocumentedTables lists schema tables no document claims.
func UndocumentedTables(root string) []string {
	used := map[string]bool{}
	for corpus := range required {
		_ = walkCorpus(root, corpus, func(_ string, fm Frontmatter, _ []string) {
			for _, t := range fm.List(LexEntities) {
				used[t] = true
			}
		})
	}
	var out []string
	for _, t := range ReadLexicon(root, LexEntities).Terms {
		if !used[t] {
			out = append(out, t)
		}
	}
	return out
}

// vendored are trees that contain SQL belonging to something other than this
// project's schema: a vendored SDK ships migration fixtures, and reading them
// puts tables in the lexicon that no table of this project has.
var vendored = []string{".gofi", ".git", "node_modules", "vendor", "testdata"}

// SchemaTables reads the table names the migrations create. This is the
// authoritative lexicon of entities: it comes from the schema, not from prose.
func SchemaTables(root string) []string {
	seen := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			for _, skip := range vendored {
				if d.Name() == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".sql") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range reCreateTable.FindAllStringSubmatch(string(raw), -1) {
			seen[strings.ToLower(m[1])] = true
		}
		return nil
	})
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
