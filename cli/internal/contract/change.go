package contract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The templates a change is created from (0003, S3). They are the project's,
// under .claude/templates/; the design comes from the pack of the backend
// language when it has one, so the how is written in that stack's terms.
const (
	ProposalTemplate = "proposal-template.md"
	DeltaTemplate    = "delta-template.md"
	DesignTemplate   = "design-template.md"
)

var (
	reChangeID  = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	reContextID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// NewChange describes a change to create.
type NewChange struct {
	ID       string
	Contexts []string
	// Language picks the design template of the pack; empty takes the neutral one.
	Language string
	// NoDesign creates the change without a design, declaring design: n/a —
	// for a change that touches no code.
	NoDesign bool
	Date     string // YYYY-MM-DD, for the templates' atualizado
}

// Create writes the folder of a new change, specs/changes/<id>/, from the
// project's templates: the proposal, a delta per context and the design. It
// returns the paths written, relative to root. It refuses an id that is not
// kebab-case or that an open or archived change already has.
func Create(root string, c NewChange) ([]string, error) {
	if !reChangeID.MatchString(c.ID) {
		return nil, fmt.Errorf("%q não é um id de mudança: kebab-case começando por verbo (adicionar-rate-limit)", c.ID)
	}
	if len(c.Contexts) == 0 {
		return nil, errors.New("informe ao menos um contexto que a mudança toca")
	}
	contexts := append([]string{}, c.Contexts...)
	sort.Strings(contexts)
	for i, ctx := range contexts {
		if !reContextID.MatchString(ctx) {
			return nil, fmt.Errorf("%q não é um nome de contexto (minúsculas, dígitos, - e _)", ctx)
		}
		if i > 0 && contexts[i-1] == ctx {
			return nil, fmt.Errorf("contexto %s repetido", ctx)
		}
	}
	dir := layout.Specs().Abs(root, ChangesDir, c.ID)
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("a mudança %s já existe em %s", c.ID, layout.Specs().Path(ChangesDir, c.ID))
	}
	if archived(root, c.ID) {
		return nil, fmt.Errorf("a mudança %s já foi arquivada: escolha outro id", c.ID)
	}

	fill := strings.NewReplacer(
		"{mudanca}", c.ID,
		"{contextos}", strings.Join(contexts, ", "),
		"{YYYY-MM-DD}", c.Date,
	)
	files := map[string]string{}
	proposal, err := readTemplate(root, layout.Templates().Abs(root, ProposalTemplate))
	if err != nil {
		return nil, err
	}
	if c.NoDesign {
		proposal = setDesignNA(proposal)
	}
	files["proposal.md"] = fill.Replace(proposal)
	delta, err := readTemplate(root, layout.Templates().Abs(root, DeltaTemplate))
	if err != nil {
		return nil, err
	}
	for _, ctx := range contexts {
		files[filepath.Join("deltas", ctx+".md")] = strings.ReplaceAll(fill.Replace(delta), "{contexto}", ctx)
	}
	if !c.NoDesign {
		design, err := readTemplate(root, designTemplate(root, c.Language))
		if err != nil {
			return nil, err
		}
		files["design.md"] = strings.ReplaceAll(fill.Replace(design), "{contexto}", contexts[0])
	}

	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var written []string
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(p, []byte(files[n]), 0o644); err != nil {
			return written, err
		}
		rel, _ := filepath.Rel(root, p)
		written = append(written, filepath.ToSlash(rel))
	}
	return written, nil
}

// designTemplate is the pack's design template for a language, or the
// neutral one when the pack has none.
func designTemplate(root, lang string) string {
	if lang != "" {
		p := layout.SDK().Abs(root, lang, "templates", DesignTemplate)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return layout.Templates().Abs(root, DesignTemplate)
}

func readTemplate(root, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		rel, _ := filepath.Rel(root, path)
		return "", fmt.Errorf("template %s ausente — rode `gofi update templates`", filepath.ToSlash(rel))
	}
	return string(b), nil
}

// setDesignNA declares, in a proposal's frontmatter, that the change has no
// design.
func setDesignNA(doc string) string {
	out, err := setFields(doc, [][2]string{{"design", "n/a"}})
	if err != nil {
		return doc
	}
	return out
}

// Change is an open change as listed: its documents and where they stand.
type Change struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	Contexts []string `json:"contextos"`
	// Sealed is true when every document of the change is approved as it is.
	Sealed bool `json:"selada"`
	// Stale lists the documents whose approval no longer holds.
	Stale []string `json:"vencidos,omitempty"`
	Docs  []string `json:"documentos"`
}

// Changes lists the open changes, sorted by id.
func Changes(all []Doc) []Change {
	byID := map[string]*Change{}
	for _, d := range all {
		if d.Change == "" {
			continue
		}
		c := byID[d.Change]
		if c == nil {
			c = &Change{ID: d.Change, Sealed: true}
			byID[d.Change] = c
		}
		c.Docs = append(c.Docs, d.Path)
		if d.Kind == KindChange {
			c.Status = value(d.Front, "status")
			c.Contexts = d.Contexts
		}
		switch state, _ := SealOf(d); state {
		case Unsealed:
			c.Sealed = false
		case Stale:
			c.Sealed = false
			c.Stale = append(c.Stale, d.Path)
		}
	}
	out := make([]Change, 0, len(byID))
	for _, id := range sortedKeys(byID) {
		out = append(out, *byID[id])
	}
	return out
}
