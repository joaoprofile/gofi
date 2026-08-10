// Package audit reports where a project's on-disk structure lags behind the
// conventions the current CLI writes.
//
// After `gofi init` the project belongs to the team, and the update family only
// refreshes what it explicitly owns: the skills, the SDK layer, the graph and
// the institutional mirror. Everything else drifts silently — .gofi.yaml
// (written once by init), specs/ and prd/ (authored by the agents),
// .claude/knowledge/ and the rest of the tree (edited by hand). A project
// scaffolded months ago therefore keeps working while missing config blocks,
// document frontmatter and knowledge files that everything newer assumes exist.
//
// Nothing here writes: the audit reports and hands the fix to the user. Every
// hint names the command that closes the finding, or says plainly that no
// command does.
package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// Severity separates "this will break something" from "this is merely old".
type Severity int

const (
	// SeverityInfo marks drift that still works but no longer matches what a
	// fresh scaffold produces.
	SeverityInfo Severity = iota
	// SeverityWarn marks drift that breaks a documented workflow — a retrieval
	// protocol that cannot run, a skill pointing at a file that is not there.
	SeverityWarn
)

func (s Severity) String() string {
	if s == SeverityWarn {
		return "warn"
	}
	return "info"
}

// Finding is one row of the audit report.
type Finding struct {
	Area     string // config | specs | prd | claude | graph
	Item     string // the file or key the finding is about
	Detail   string // what is off
	Hint     string // what closes it
	Severity Severity
}

// Options carries what the checks cannot derive from the project itself.
type Options struct {
	// Upstream is the fetched agents source, used to tell which
	// ai/knowledge/shared/ files the project is missing. Nil skips that check.
	Upstream fs.FS
	// UpstreamRoot is the prefix of the agents tree inside Upstream (usually ".").
	UpstreamRoot string
	// GraphEnabled is the caller's reading of the graph block, whose absence
	// means enabled.
	GraphEnabled bool
}

// Run inspects the project rooted at root and returns every finding, ordered by
// area so the report reads as a walk through the project.
//
// The config checks read .gofi.yaml from disk rather than the parsed struct,
// because config.Load upgrades the schema in memory and would otherwise hide
// the very fact this audit is looking for.
func Run(root string, opts Options) []Finding {
	var out []Finding
	out = append(out, checkConfig(root)...)
	out = append(out, checkDocs(root, "specs")...)
	out = append(out, checkDocs(root, "prd")...)
	out = append(out, checkClaude(root, opts)...)
	out = append(out, checkGraph(root, opts.GraphEnabled)...)
	return out
}

// checkConfig compares .gofi.yaml against the schema version and the block set
// the current init writes. Blocks are checked by key presence rather than by the
// parsed struct: an absent `sonar:` and a present-but-empty one mean different
// things to the user, and only the raw document tells them apart.
func checkConfig(root string) []Finding {
	file := filepath.Join(root, config.FileName)
	data, err := os.ReadFile(file)
	if err != nil {
		return []Finding{{
			Area: "config", Item: config.FileName, Severity: SeverityWarn,
			Detail: "not readable: " + err.Error(),
			Hint:   "run gofi init at the project root",
		}}
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return []Finding{{
			Area: "config", Item: config.FileName, Severity: SeverityWarn,
			Detail: "not valid YAML: " + err.Error(),
			Hint:   "fix it by hand, then run gofi config",
		}}
	}

	var out []Finding
	if v, ok := doc["version"].(int); ok && v < config.CurrentVersion {
		out = append(out, Finding{
			Area: "config", Item: "version", Severity: SeverityWarn,
			Detail: sprintVersion(v),
			Hint:   "gofi config --wizard rewrites it in the current schema; no update touches this file",
		})
	}

	// Blocks added after the first releases. A project created before each of
	// them keeps running — the CLI defaults them in memory — but the file no
	// longer describes what the project actually does. The list comes from
	// config so the report and the repair can never disagree about what a
	// current project is supposed to carry.
	for _, key := range config.MissingBlocks(file) {
		out = append(out, Finding{
			Area: "config", Item: key + ":", Severity: SeverityInfo,
			Detail: "block absent — " + blockPurpose[key],
			Hint:   "add it by hand, or gofi config --wizard rewrites the file with the current defaults",
		})
	}
	out = append(out, checkUISurfaces(doc)...)
	return out
}

// checkUISurfaces reports surface keys written as a block where the schema reads
// a single string. This is the one drift that stops the config from loading at
// all, which makes the audit the only command left that can name the key — the
// yaml error alone gives a line number and nothing else.
func checkUISurfaces(doc map[string]any) []Finding {
	var out []Finding
	for _, block := range []string{"frontend", "mobile", "surfaces", "ui"} {
		if m, ok := doc[block].(map[string]any); ok {
			out = append(out, scanSurface(block, m)...)
		}
	}
	return out
}

func scanSurface(where string, m map[string]any) []Finding {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []Finding
	for _, k := range keys {
		nested, isBlock := m[k].(map[string]any)
		if isBlock && (nested["framework"] != nil || nested["path"] != nil) {
			// A sub-surface (ui.web, ui.mobile, …), not a mistyped field.
			out = append(out, scanSurface(where+"."+k, nested)...)
			continue
		}
		if !config.SurfaceFields[k] {
			continue
		}
		_, isList := m[k].([]any)
		if !isBlock && !isList {
			continue
		}
		// brand is the one block form the old gofi-ui material documented, so
		// the loader folds it to its `surface` value and the config still loads.
		// Any other field written as a block was never documented that way and
		// stops the config from loading.
		f := Finding{
			Area: "config", Item: where + "." + k, Severity: SeverityWarn,
			Detail: "written as a block; the schema reads it as a single string",
			Hint:   "collapse it to a single value — no gofi command runs until the config loads",
		}
		if k == "brand" {
			f.Severity = SeverityInfo
			f.Hint = "read as its `surface` value; write it as that single string — the other roles are derived from it"
		}
		out = append(out, f)
	}
	return out
}

// blockPurpose says what each block is for, so a finding explains the gap
// instead of only naming it.
var blockPurpose = map[string]string{
	"hsec":      "Horusec SAST settings (gofi hsec)",
	"sonar":     "SonarQube settings (gofi sonar)",
	"test":      "test tasks (gofi test)",
	"ops":       "platform/delivery settings (gofi-ops)",
	"graph":     "code graph settings — chiefly the scan mode, which decides what the agents may conclude from the graph",
	"ai.models": "the model list the panel's /model picker offers, which falls back to built-ins without it",
}

func sprintVersion(v int) string {
	return fmt.Sprintf("schema v%d, current is v%d", v, config.CurrentVersion)
}

// checkDocs walks specs/ or prd/ and reports documents the RAG retrieval
// protocol cannot index. Discovery there works off the frontmatter alone, so a
// document without one is invisible to every agent that follows the protocol —
// it is not merely untidy.
func checkDocs(root, dir string) []Finding {
	base := filepath.Join(root, dir)
	if _, err := os.Stat(base); err != nil {
		return nil // a project without specs yet is not a drifting project
	}

	var out []Finding
	var docs int
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".md") || name == "INDEX.md" {
			return nil
		}
		docs++
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		out = append(out, inspectDoc(rel, string(body))...)
		return nil
	})

	if docs > 0 {
		if _, err := os.Stat(filepath.Join(base, "INDEX.md")); err != nil {
			out = append(out, Finding{
				Area: dir, Item: dir + "/INDEX.md", Severity: SeverityWarn,
				Detail: "missing — agents discover documents through it",
				Hint:   "bash .claude/scripts/gen-index.sh " + dir,
			})
		}
	}
	return out
}

// legacyMarkers are the provenance blocks the pre-RAG templates carried in the
// body. They are not wrong, only superseded: the frontmatter and git hold the
// same facts, and the agents were told to stop emitting them.
var legacyMarkers = []string{"**Autor:**", "**Versão:**", "**Data:**", "## Rastreabilidade"}

func inspectDoc(rel, body string) []Finding {
	front, ok := frontmatter(body)
	if !ok {
		return []Finding{{
			Area: area(rel), Item: rel, Severity: SeverityWarn,
			Detail: "no YAML frontmatter — invisible to the retrieval protocol",
			Hint:   "add the frontmatter from .claude/templates/, then regenerate the INDEX",
		}}
	}

	var out []Finding
	var missing []string
	for _, key := range []string{"versao", "keywords", "status", "contexto"} {
		if !hasKey(front, key) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		out = append(out, Finding{
			Area: area(rel), Item: rel, Severity: SeverityWarn,
			Detail: "frontmatter without " + strings.Join(missing, ", "),
			Hint:   "gen-index.sh skips documents missing contexto/keywords",
		})
	}
	for _, m := range legacyMarkers {
		if strings.Contains(body, m) {
			out = append(out, Finding{
				Area: area(rel), Item: rel, Severity: SeverityInfo,
				Detail: "carries the old provenance block (" + m + ")",
				Hint:   "provenance lives in the frontmatter and in git now",
			})
			break
		}
	}
	return out
}

func area(rel string) string {
	if i := strings.IndexByte(rel, '/'); i > 0 {
		return rel[:i]
	}
	return rel
}

// frontmatter returns the block between the leading --- and the next ---.
func frontmatter(body string) (string, bool) {
	body = strings.TrimLeft(body, "\ufeff\r\n ")
	if !strings.HasPrefix(body, "---") {
		return "", false
	}
	rest := body[3:]
	rest = strings.TrimLeft(rest, "\r")
	if !strings.HasPrefix(rest, "\n") {
		return "", false
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}
	return rest[1:end], true
}

func hasKey(front, key string) bool {
	for line := range strings.Lines(front) {
		if strings.HasPrefix(strings.TrimSpace(line), key+":") {
			return true
		}
	}
	return false
}

// checkClaude reports what .claude/ is missing or still carries from an older
// layout. Only skills/ and sdk/ have a command that refreshes them; the rest of
// the tree is installed once by `gofi init` and belongs to the team from then
// on, so every finding here names what actually closes it rather than pointing
// at an update that would not touch the file.
func checkClaude(root string, opts Options) []Finding {
	claude := filepath.Join(root, ".claude")
	if _, err := os.Stat(claude); err != nil {
		return []Finding{{
			Area: "claude", Item: ".claude/", Severity: SeverityWarn,
			Detail: "missing — the agents have nothing to read",
			Hint:   "gofi update skills restores the skills; the rest of the tree comes from gofi init",
		}}
	}

	var out []Finding
	if _, err := os.Stat(filepath.Join(claude, "scripts", "gen-index.sh")); err != nil {
		out = append(out, Finding{
			Area: "claude", Item: ".claude/scripts/gen-index.sh", Severity: SeverityInfo,
			Detail: "absent — INDEX regeneration is manual",
			Hint:   "copy it from ai/scripts/ in the gofi repo; no update installs it",
		})
	}
	out = append(out, legacyLayout(claude)...)
	out = append(out, missingShared(claude, opts)...)
	return out
}

// legacyLayout reports the pre-v2.4 SDK directories: a flat .claude/boilerplates/
// and sdk-knowledge/, and the .claude/gofi-sdk-<lang>/ that preceded
// .claude/sdk/<lang>/. They are dead weight the agents may still read, which is
// worse than absent — two copies of the same doc, one of them frozen.
func legacyLayout(claude string) []Finding {
	var stale []string
	for _, name := range []string{"boilerplates", "sdk-knowledge"} {
		if info, err := os.Stat(filepath.Join(claude, name)); err == nil && info.IsDir() {
			stale = append(stale, ".claude/"+name+"/")
		}
	}
	entries, _ := os.ReadDir(claude)
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "gofi-sdk-") {
			stale = append(stale, ".claude/"+e.Name()+"/")
		}
	}
	if len(stale) == 0 {
		return nil
	}
	sort.Strings(stale)
	return []Finding{{
		Area: "claude", Item: ".claude/", Severity: SeverityWarn,
		Detail: "pre-v2.4 SDK dir(s) beside the current tree: " + strings.Join(stale, ", "),
		Hint:   "gofi update sdk removes them",
	}}
}

func missingShared(claude string, opts Options) []Finding {
	if opts.Upstream == nil {
		return nil
	}
	srcRoot := opts.UpstreamRoot
	if srcRoot == "" {
		srcRoot = "."
	}
	entries, err := fs.ReadDir(opts.Upstream, path.Join(srcRoot, "ai", "knowledge", "shared"))
	if err != nil {
		return nil
	}
	var missing []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if _, err := os.Stat(filepath.Join(claude, "knowledge", "shared", e.Name())); err != nil {
			missing = append(missing, e.Name())
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return []Finding{{
		Area: "claude", Item: ".claude/knowledge/shared/", Severity: SeverityWarn,
		Detail: "missing upstream file(s): " + strings.Join(missing, ", "),
		Hint:   "no update writes knowledge/; copy them from ai/knowledge/shared/ to get the protocols the skills cite",
	}}
}

func checkGraph(root string, enabled bool) []Finding {
	if !enabled {
		return nil
	}
	if _, err := os.Stat(filepath.Join(root, ".gofi", "graph")); err != nil {
		return []Finding{{
			Area: "graph", Item: ".gofi/graph/", Severity: SeverityInfo,
			Detail: "graph enabled but never built",
			Hint:   "gofi graph build",
		}}
	}
	return nil
}
