package layout

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/host"
)

// DefaultHome is the folder the harness keeps a project's agent content in
// when nothing says otherwise.
const DefaultHome = ".claude"

// home is this project's folder. It is set once, when the CLI reads the
// project's .gofi.yaml, before anything reads a path — see SetHome.
var home = DefaultHome

// HomeFor names the folder a host discovers agent content in. Claude Code only
// looks in .claude/; Codex, Copilot and the tools converging on the open
// skills convention look in .agents/. One project, one folder: gofi init
// picks it from the host, and a change of host is a migration, never a mirror.
func HomeFor(id string) string {
	if h, ok := host.Get(id); ok {
		return h.Home
	}
	return DefaultHome
}

// SetHome fixes the folder for this process. Called once at start-up; every
// path below is computed from it when it is asked for, so nothing captured a
// default before the project was read.
func SetHome(h string) {
	if h != "" {
		home = h
	}
}

// Home is where the harness keeps, inside this project, everything the agents
// read: skills and their references, templates, SDK docs, and the project's
// own knowledge, memory and vocabulary.
func Home() string { return home }

// Owner is who a project area belongs to, and so who may rewrite it.
type Owner string

const (
	// OwnerGofi areas come from upstream: `gofi update` refreshes them, and a
	// team that needs something different says so in knowledge/, not by
	// editing them.
	OwnerGofi Owner = "gofi"
	// OwnerProject areas are the team's: gofi reads, indexes and validates
	// them, and never rewrites them.
	OwnerProject Owner = "project"
)

// Area is one part of a project the harness knows, and who owns it.
type Area struct {
	Name  string
	Dir   string // slash-separated, relative to the project root
	Owner Owner
}

// Path joins elem under the area, slash-separated and project-relative: the
// form paths take in documents, indexes and messages.
func (a Area) Path(elem ...string) string {
	return path.Join(append([]string{a.Dir}, elem...)...)
}

// Abs is Path under a project root, in the platform's form.
func (a Area) Abs(root string, elem ...string) string {
	return filepath.Join(root, filepath.FromSlash(a.Path(elem...)))
}

// The areas. Their owners are the contract every update, audit and doctor
// check reads — declared here once, so no command decides it on its own.
func Skills() Area        { return Area{"skills", home + "/skills", OwnerGofi} }
func Templates() Area     { return Area{"templates", home + "/templates", OwnerGofi} }
func SDK() Area           { return Area{"sdk", home + "/sdk", OwnerGofi} }
func Expertise() Area     { return Area{"expertise", home + "/expertise", OwnerGofi} }
func Knowledge() Area     { return Area{"knowledge", home + "/knowledge", OwnerProject} }
func Institutional() Area { return Area{"institutional", home + "/institutional", OwnerProject} }
func Memory() Area        { return Area{"memory", home + "/memory", OwnerProject} }
func Lexicon() Area       { return Area{"lexicon", home + "/lexicon", OwnerProject} }
func Eval() Area          { return Area{"eval", home + "/eval", OwnerProject} }
func Specs() Area         { return Area{"specs", "specs", OwnerProject} }
func PRD() Area           { return Area{"prd", "prd", OwnerProject} }

// Contexts is where each context keeps its state: part of memory, and a
// document corpus of its own.
func Contexts() Area { return Area{"contexts", Memory().Path("contexts"), OwnerProject} }

// Areas lists every area, gofi's first.
func Areas() []Area {
	return []Area{Skills(), Templates(), SDK(), Expertise(), Knowledge(), Institutional(), Memory(), Lexicon(), Eval(), Specs(), PRD()}
}

// OwnerOf says who owns a project-relative path, by the area it sits in.
func OwnerOf(rel string) (Owner, bool) {
	rel = filepath.ToSlash(rel)
	for _, a := range Areas() {
		if rel == a.Dir || strings.HasPrefix(rel, a.Dir+"/") {
			return a.Owner, true
		}
	}
	return "", false
}
