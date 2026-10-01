package docs

import (
	"bytes"
	"fmt"
	"github.com/gofi-labs/gofi/cli/internal/expertise"
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The retrieval index a person and an agent read, as opposed to the JSON the
// tool reads. Two levels on purpose.
const (
	IndexMarkdown  = "INDEX.md"
	signatureTerms = 14
)

// KnowledgeIndex is the generated manifest of the knowledge layers.
func KnowledgeIndex() string { return layout.Knowledge().Path("INDEX.md") }

// Drift is a corpus layout problem worth reporting but not worth failing on.
type Drift string

// writeIndexes writes the two-level retrieval index for a corpus: the router
// and the shards.
//
// One level answers "which context", the other "which documents are here".
// Splitting them is what keeps the entry cost flat: contexts grow slowly,
// documents do not, so a single table would grow without bound while still only
// ever leading to one document.
//
// The shard follows the DIRECTORY, not the context. A context that grew
// submodules spreads across folders — product covers product_pim,
// product_visits and more — and tying the shard to the context left those
// folders with no index at all, while the index describing them sat in a
// sibling. Directory is physical organisation, context is the concept, and
// submodulo is the link between them; the router carries that mapping.
func writeIndexes(root, corpus string, out *[]string) ([]Drift, error) {
	if st, err := os.Stat(filepath.Join(root, corpus)); err != nil || !st.IsDir() {
		return nil, nil
	}
	byContext, err := groupByContext(root, corpus)
	if err != nil || len(byContext) == 0 {
		return nil, err
	}
	signature := signatures(byContext)

	byDir := map[string][]indexEntry{}
	dirsOf := map[string][]string{}
	for ctx, group := range byContext {
		seen := map[string]bool{}
		for _, e := range group {
			byDir[e.dir] = append(byDir[e.dir], e)
			if !seen[e.dir] {
				seen[e.dir] = true
				dirsOf[ctx] = append(dirsOf[ctx], e.dir)
			}
		}
		sort.Strings(dirsOf[ctx])
	}

	contexts := make([]string, 0, len(byContext))
	for c := range byContext {
		contexts = append(contexts, c)
	}
	sort.Strings(contexts)

	written := map[string]bool{}
	var drift []Drift
	var b strings.Builder
	total := 0
	fmt.Fprintf(&b, "# Índice de %s — roteador de contextos\n\n", corpusTitle(corpus))
	b.WriteString(`> **Nível 1 de 2.** Responde **só uma pergunta**: *qual contexto?* Achou, abra os
> índices das pastas dele — é lá que estão os documentos. Um contexto com
> submódulos ocupa mais de uma pasta; todas estão listadas.
>
> **Não é um portão.** Os termos abaixo são assinatura, não catálogo: a ausência
> de um termo não significa ausência do assunto. Na dúvida entre dois ou três,
> **abra os dois ou três** — errar o palpite sai mais barato que adivinhar bem.
> Melhor ainda: ` + "`gofi find \"<pergunta>\"`" + `, que pergunta ao corpus inteiro.
>
> Derivado — não edite à mão. Regenere com ` + "`gofi index docs`" + `.

| Contexto | Docs | Do que trata | Onde |
|---|--:|---|---|
`)
	for _, ctx := range contexts {
		group := byContext[ctx]
		total += len(group)
		locais := make([]string, 0, len(dirsOf[ctx]))
		for _, d := range dirsOf[ctx] {
			locais = append(locais, fmt.Sprintf("`%s/%s`", d, IndexMarkdown))
		}
		fmt.Fprintf(&b, "| %s | %d | %s | %s |\n",
			ctx, len(group), strings.Join(signature[ctx], ", "), strings.Join(locais, "<br>"))
	}
	fmt.Fprintf(&b, "\n_%d documentos em %d contextos, %d pastas._\n",
		total, len(contexts), len(byDir))

	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		shardPath := filepath.Join(root, filepath.FromSlash(dir), IndexMarkdown)
		written[shardPath] = true
		content, d := renderShard(dir, corpus, byDir[dir])
		if d != "" {
			drift = append(drift, d)
		}
		if err := writeIfChanged(shardPath, []byte(content), out); err != nil {
			return drift, err
		}
	}

	path := filepath.Join(root, corpus, IndexMarkdown)
	written[path] = true
	if err := writeIfChanged(path, []byte(b.String()), out); err != nil {
		return drift, err
	}
	return append(drift, sweepShards(root, corpus, written, out)...), nil
}

type indexEntry struct {
	rel, dir, ctx, sub, version, status          string
	entities, operations, keywords, marketplaces []string
}

func groupByContext(root, corpus string) (map[string][]indexEntry, error) {
	out := map[string][]indexEntry{}
	err := walkCorpus(root, corpus, func(path string, fm Frontmatter, _ []string) {
		rel := relPath(root, path)
		ctx := fm.Get("contexto")
		if ctx == "" {
			ctx = "sem-contexto"
		}
		sub := fm.Get("submodulo")
		if sub == "" {
			sub = "n/a"
		}
		out[ctx] = append(out[ctx], indexEntry{
			rel: rel, dir: filepath.ToSlash(filepath.Dir(rel)), ctx: ctx, sub: sub,
			version: fm.Get("versao"), status: fm.Get("status"),
			entities: fm.List("entidades"), operations: fm.List("operacoes"),
			keywords: fm.List("keywords"), marketplaces: fm.List("marketplaces"),
		})
	})
	return out, err
}

func renderShard(dir, corpus string, group []indexEntry) (string, Drift) {
	sort.Slice(group, func(i, j int) bool {
		if group[i].sub != group[j].sub {
			return group[i].sub < group[j].sub
		}
		return group[i].rel < group[j].rel
	})
	ctxs := map[string]bool{}
	for _, e := range group {
		ctxs[e.ctx] = true
	}
	nomes := make([]string, 0, len(ctxs))
	for c := range ctxs {
		nomes = append(nomes, c)
	}
	sort.Strings(nomes)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s — %s\n\n", dir, corpusTitle(corpus))
	fmt.Fprintf(&b, "> **Nível 2 de 2.** Documentos desta pasta. Contexto: **%s**.\n",
		strings.Join(nomes, ", "))
	b.WriteString(`> **Entidades** são tabelas reais do schema — se procura quem define ou usa uma
> tabela, é esta coluna. Escolhido o documento, leia o frontmatter e pule para a
> §seção; nunca o arquivo inteiro.
>
> Derivado — não edite à mão. Regenere com ` + "`gofi index docs`" + `.

| Submódulo | Versão | Status | Entidades | Assunto | Arquivo |
|---|---|---|---|---|---|
`)
	for _, e := range group {
		entities := strings.Join(e.entities, ", ")
		if entities == "" {
			entities = "—"
		}
		subject := uniq(append(append(append([]string{}, e.operations...), e.keywords...), e.marketplaces...))
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | `%s` |\n",
			e.sub, e.version, e.status, entities, strings.Join(subject, ", "), e.rel)
	}

	// A folder named after the context, or after the context and one of its
	// submodules, is deliberate layout — a context that grew submodules spreads
	// across folders and that is the point. Anything else is a typo, the kind
	// that makes a path unguessable, and only that is worth reporting.
	if len(nomes) == 1 && !folderExplained(dir, corpus, nomes[0], group) {
		return b.String(), Drift(fmt.Sprintf("pasta %q serve só o contexto %q e o nome não corresponde "+
			"nem ao contexto nem a um submódulo dela", dir, nomes[0]))
	}
	return b.String(), ""
}

// folderExplained reports whether the folder name is accounted for by the
// context it serves or by a submodule inside it.
func folderExplained(dir, corpus, ctx string, group []indexEntry) bool {
	base := strings.TrimPrefix(dir, corpus+"/")
	if base == ctx {
		return true
	}
	rest := strings.TrimPrefix(base, ctx)
	if rest == base {
		return false // does not even start with the context name
	}
	rest = strings.TrimLeft(rest, "-_")
	if rest == "" {
		return false
	}
	norm := func(x string) string { return strings.ReplaceAll(x, "_", "-") }
	for _, e := range group {
		if sub := norm(e.sub); sub != "" && sub != "n/a" && strings.HasPrefix(sub, norm(rest)) {
			return true
		}
	}
	return false
}

// signatures picks the terms that describe a context without being universal.
//
// NOTE ON METHOD: choosing between variants here is not decidable by a lexical
// evaluation — it systematically underrates compact surfaces, because token
// overlap between a natural question and a term list is near zero. Whoever
// ranks these in production is a model reading them, semantically. Validate
// with an LLM judge before changing this.
func signatures(byContext map[string][]indexEntry) map[string][]string {
	spread := map[string]int{}
	for _, group := range byContext {
		seen := map[string]bool{}
		for _, e := range group {
			for _, t := range append(append([]string{}, e.keywords...), e.operations...) {
				seen[t] = true
			}
		}
		for t := range seen {
			spread[t]++
		}
	}
	out := map[string][]string{}
	for ctx, group := range byContext {
		local := map[string]int{}
		for _, e := range group {
			for _, t := range append(append([]string{}, e.keywords...), e.operations...) {
				local[t]++
			}
		}
		terms := make([]string, 0, len(local))
		for t := range local {
			terms = append(terms, t)
		}
		sort.Slice(terms, func(i, j int) bool {
			a := float64(local[terms[i]]) / float64(spread[terms[i]])
			b := float64(local[terms[j]]) / float64(spread[terms[j]])
			if a != b {
				return a > b
			}
			if local[terms[i]] != local[terms[j]] {
				return local[terms[i]] > local[terms[j]]
			}
			return terms[i] < terms[j]
		})
		if len(terms) > signatureTerms {
			terms = terms[:signatureTerms]
		}
		out[ctx] = terms
	}
	return out
}

func corpusTitle(corpus string) string {
	switch corpus {
	case "specs":
		return "Specs (SDD)"
	case "prd":
		return "PRDs"
	}
	return corpus
}

// Areas of portable knowledge a project carries, in reading order.
type knowledgeArea struct{ dir, about string }

// core is the knowledge short and universal enough to always be worth loading.
// Everything else is situational and waits to be asked for.
var core = map[string]bool{
	"absolute-rules.md": true, "naming.md": true, "layers.md": true,
	"structure.md": true, "ddd-principles.md": true, "clean-code.md": true,
}

// writeKnowledgeIndex writes the manifest of the portable knowledge layers.
//
// The reading convention used to say "read knowledge/shared/*.md" — a glob,
// with no way to be selective. That is the largest fixed cost in the harness,
// paid on every invocation whatever the task, and larger than the spec of the
// context being worked on. The manifest exists so an agent can load the core
// and then only the modules the task actually calls for.
func writeKnowledgeIndex(root, language string, out *[]string) error {
	var areas []knowledgeArea
	// A pack with a broken contract is left out, as it is from routing;
	// `gofi index check` reports it.
	packs, _ := expertise.Load(root)
	for _, p := range packs {
		about := p.Title
		if p.Summary != "" {
			about += " — " + p.Summary
		}
		areas = append(areas, knowledgeArea{p.Dir, about})
	}
	areas = append(areas, knowledgeArea{layout.Knowledge().Path("shared"), "Aprendizado do time — vale para todos os papéis e vence os packs quando diverge"})
	if language != "" {
		areas = append(areas,
			knowledgeArea{layout.SDK().Path(language, "knowledge"), language + " — padrões e armadilhas do SDK"},
			knowledgeArea{layout.SDK().Path(language, "api"), language + " — API do SDK por pacote (gerada do código)"},
			knowledgeArea{layout.SDK().Path(language, "boilerplates"), language + " — esqueletos por camada"},
		)
	}

	var b strings.Builder
	b.WriteString(`# Índice do conhecimento portável — carregue por necessidade

> **Não carregue estas pastas por glob.** Ler tudo em toda invocação é o maior
> custo fixo do harness — maior que a spec do contexto em que se está mexendo.
>
> **Núcleo ⬤** é curto e universal: carregue sempre. **O resto é sob demanda**:
> leia a linha *Quando* e carregue só o que a tarefa pede.
>
> Derivado — regenere com ` + "`gofi index docs`" + ` após ` + "`gofi update sdk`" + `.

`)
	var totalLines, totalBytes, coreLines, coreBytes int
	for _, area := range areas {
		entries, err := os.ReadDir(filepath.Join(root, area.dir))
		if err != nil {
			continue
		}
		var files []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != IndexMarkdown && e.Name() != expertise.ManifestFile {
				files = append(files, e.Name())
			}
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		fmt.Fprintf(&b, "## `%s/`\n\n_%s_\n\n| | Arquivo | Linhas | Quando |\n|---|---|--:|---|\n",
			area.dir, area.about)
		for _, name := range files {
			raw, err := os.ReadFile(filepath.Join(root, area.dir, name))
			if err != nil {
				continue
			}
			n, size := len(strings.Split(string(raw), "\n")), len(raw)
			totalLines, totalBytes = totalLines+n, totalBytes+size
			mark := ""
			if core[name] {
				mark, coreLines, coreBytes = "⬤", coreLines+n, coreBytes+size
			}
			fmt.Fprintf(&b, "| %s | `%s` | %d | %s |\n", mark, name, n, firstProse(string(raw)))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, `## Custo

| | Linhas | ~Tokens |
|---|--:|--:|
| Tudo (glob) | %d | ~%dk |
| Núcleo ⬤ | %d | ~%dk |
| Núcleo + 2 módulos | ~%d | ~%dk |
`, totalLines, totalBytes/3600, coreLines, coreBytes/3600,
		coreLines+600, (coreBytes+21000)/3600)

	return writeIfChanged(filepath.Join(root, filepath.FromSlash(KnowledgeIndex())), []byte(b.String()), out)
}

// firstProse is the first sentence that is not a heading, quote, table, list or
// fence — enough to tell an agent whether the file is worth opening.
func firstProse(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				lines = lines[i+1:]
				break
			}
		}
	}
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		switch t[0] {
		case '#', '>', '|', '-', '*', '`':
			continue
		}
		t = strings.NewReplacer("`", "", "*", "", "[", "", "]", "").Replace(t)
		if len(t) > 96 {
			t = t[:96] + "…"
		}
		return t
	}
	return ""
}

// generatedMark identifies a shard this tool wrote. A hand-written INDEX.md is
// somebody's file and is never touched.
const generatedMark = "Derivado — não edite à mão"

// sweepShards deletes indexes left behind by a context that no longer exists.
//
// Merging or renaming a context used to leave its old shard in place forever,
// still announcing a context the corpus no longer has. That is the failure this
// whole index exists to avoid: an index nobody rebuilt is not merely unhelpful,
// it answers confidently and wrongly.
func sweepShards(root, corpus string, written map[string]bool, out *[]string) []Drift {
	var removed []Drift
	base := filepath.Join(root, corpus)
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != IndexMarkdown || written[path] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(raw), generatedMark) {
			return nil // not ours
		}
		if os.Remove(path) == nil {
			if out != nil {
				*out = append(*out, path)
			}
			removed = append(removed, Drift(fmt.Sprintf(
				"removido índice órfão %s — o contexto que ele descrevia não existe mais",
				relPath(root, path))))
		}
		return nil
	})
	return removed
}

// writeIfChanged writes only when the content differs, so an unchanged index
// is not rewritten (and does not show up as modified). Paths actually written
// are appended to written.
func writeIfChanged(path string, data []byte, written *[]string) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if written != nil {
		*written = append(*written, path)
	}
	return nil
}
