// Package expertise reads the expertise packs a project has: what each one
// covers, and the signals that say a task needs it.
//
// A pack is one coherent domain — DDD modelling, Postgres persistence, Kafka
// messaging — written once and shared by every role that uses it: the spec
// models with it, the implementation writes with it, the audit checks with it.
// Packs are not skills: a host lists every skill in every session, and with
// dozens of lookalike descriptions the model picks worse. The router picks
// packs instead, from signals the index can verify, and says why.
package expertise

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// ManifestFile is the file that makes a folder a pack: its contract in the
// frontmatter, an overview in the body.
const ManifestFile = "PACK.md"

// Pack is one expertise pack.
type Pack struct {
	Name    string      `yaml:"pack"`
	Title   string      `yaml:"title"`
	Summary string      `yaml:"summary"`
	When    AppliesWhen `yaml:"applies_when"`
	// Serves names the roles that use the pack: pd, spec, eng, ui, ops, qa, doc.
	Serves []string `yaml:"serves"`

	// Dir is the pack's folder, project-relative and slash-separated.
	Dir string `yaml:"-"`
}

// AppliesWhen is what makes a task need a pack. Each list is a set of signals
// the index can check; any one matching is enough.
type AppliesWhen struct {
	// Always selects the pack for every task of the roles it serves: the
	// protocols every role follows, whatever the task touches.
	Always bool `yaml:"always"`
	// Imports are packages the code a task touches imports (gofi/msq).
	Imports []string `yaml:"imports"`
	// Symbols are name patterns of symbols the task touches (*Consumer).
	Symbols []string `yaml:"symbols"`
	// Intents are the verbs of the request (integrar, consumir).
	Intents []string `yaml:"intents"`
	// Entities are kinds of entity the task deals with (evento, tabela).
	Entities []string `yaml:"entities"`
	// Paths are globs of files the task touches (**/migrations/*.sql).
	Paths []string `yaml:"paths"`
}

// Empty reports a contract with no signal at all: a pack nothing can select.
func (w AppliesWhen) Empty() bool {
	return !w.Always && len(w.Imports)+len(w.Symbols)+len(w.Intents)+len(w.Entities)+len(w.Paths) == 0
}

// Roles are the values Serves may take.
var Roles = []string{"pd", "spec", "eng", "ui", "ops", "qa", "doc"}

// Problem is something wrong with a pack, reported by `gofi index check`.
type Problem struct {
	Path    string
	Message string
}

func (p Problem) String() string { return p.Path + ": " + p.Message }

// Load reads every pack of a project, sorted by name. A pack that cannot be
// read is reported, not returned: a broken contract must not route anything.
func Load(root string) ([]Pack, []Problem) {
	base := layout.Expertise().Abs(root)
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []Problem{{layout.Expertise().Dir, err.Error()}}
	}
	var packs []Pack
	var problems []Problem
	seen := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rel := layout.Expertise().Path(e.Name(), ManifestFile)
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			problems = append(problems, Problem{rel, "missing — a folder under expertise/ needs a " + ManifestFile})
			continue
		}
		p, errs := Parse(b)
		p.Dir = layout.Expertise().Path(e.Name())
		if p.Name != "" && p.Name != e.Name() {
			errs = append(errs, fmt.Sprintf("pack %q does not match its folder %q", p.Name, e.Name()))
		}
		if prev, dup := seen[p.Name]; dup && p.Name != "" {
			errs = append(errs, "pack name also used by "+prev)
		}
		seen[p.Name] = rel
		for _, m := range errs {
			problems = append(problems, Problem{rel, m})
		}
		if len(errs) == 0 {
			packs = append(packs, p)
		}
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].Name < packs[j].Name })
	return packs, problems
}

// Parse reads a pack's contract from its PACK.md and checks it. Unknown keys
// are errors: a misspelt signal would otherwise be a rule that silently never
// fires.
func Parse(b []byte) (Pack, []string) {
	var p Pack
	front, ok := frontmatter(b)
	if !ok {
		return p, []string{"no frontmatter — the contract lives between --- lines at the top"}
	}
	dec := yaml.NewDecoder(bytes.NewReader(front))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return p, []string{"contract: " + err.Error()}
	}
	var errs []string
	if p.Name == "" {
		errs = append(errs, "pack: required")
	}
	if strings.TrimSpace(p.Title) == "" {
		errs = append(errs, "title: required")
	}
	if p.When.Empty() {
		errs = append(errs, "applies_when: no signal — nothing could ever select this pack")
	}
	if len(p.Serves) == 0 {
		errs = append(errs, "serves: name the roles that use the pack")
	}
	for _, r := range p.Serves {
		if !slices.Contains(Roles, r) {
			errs = append(errs, fmt.Sprintf("serves: %q is not a role (expected %s)", r, strings.Join(Roles, ", ")))
		}
	}
	return p, errs
}

// frontmatter returns the YAML between the leading --- lines.
func frontmatter(b []byte) ([]byte, bool) {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(b, []byte("---\n")) {
		return nil, false
	}
	rest := b[4:]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, false
	}
	return rest[:end+1], true
}
