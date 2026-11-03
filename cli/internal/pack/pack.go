// Package pack is what gofi knows about a language and nothing else does
// (decision 0002, D13): the commands that check a project in it and the
// format each one reports in. The core never names a language's tool; it
// reads the pack.
package pack

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema names the format of a pack.
const Schema = "gofi-pack/v1"

// Formats a sensor's report can be read in. One parser per format, not per
// tool: every ecosystem already emits one of these.
const (
	FormatLines      = "lines"        // file:line[:col]: message — compilers, vet
	FormatGoTestJSON = "go-test-json" // go test -json events
	FormatSARIF      = "sarif"        // static analysis, any linter
	FormatJUnit      = "junit"        // JUnit XML — jest-junit, pytest, surefire, nextest
	FormatExit       = "exit"         // only the exit code says anything
)

var formats = []string{FormatLines, FormatGoTestJSON, FormatSARIF, FormatJUnit, FormatExit}

//go:embed packs/*.yaml
var builtin embed.FS

// Pack is one language's.
type Pack struct {
	Schema   string             `yaml:"schema"`
	Language string             `yaml:"language"`
	Sensors  map[string]Command `yaml:"sensors"`
	// Tests are the names of the language's test files, as path.Match
	// patterns on a file's base name: what the loop reads as a test and not
	// as the code it tests.
	Tests []string `yaml:"tests"`
	// Sources are the names of the language's source files, tests among
	// them: what the loop guards the authorship of.
	Sources []string `yaml:"sources"`
	// Layers are what a backend in the language is divided into, by where
	// their files are; the first whose paths match a file is its layer.
	Layers []Layer `yaml:"layers,omitempty"`
	// Contexts is where a context's code lives, relative to the backend
	// folder, with {context} for its name: domain/{context}/.
	Contexts string `yaml:"contexts,omitempty"`
	// Graph is what the graph stage holds the code to.
	Graph GraphRules `yaml:"graph,omitempty"`
}

// Layer is one layer of the language's backend: the paths of its files,
// relative to the backend folder — * for one name, ** for any folders — and
// the layer of a plan its work is part of.
type Layer struct {
	Name  string   `yaml:"name"`
	Plan  string   `yaml:"plan"`
	Paths []string `yaml:"paths"`
}

// GraphRules are the rules the code graph is held to (0002, D11), each a
// line: "deny: <layer> -> <layer>", "deny: new-cycle", "deny: import
// "<package>" from <layer>".
type GraphRules struct {
	Rules []string `yaml:"rules,omitempty"`
}

// LayerOf is the layer of a file relative to the backend folder, and the
// plan's layer it is part of; empty when no layer holds it.
func (p *Pack) LayerOf(file string) (layer, plan string) {
	if p == nil {
		return "", ""
	}
	file = strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "./")
	for _, l := range p.Layers {
		for _, pat := range l.Paths {
			if Glob(pat, file) {
				return l.Name, l.Plan
			}
		}
	}
	return "", ""
}

// ContextOf is the context whose code a file is, relative to the backend
// folder: the name where the pack's contexts pattern says {context}.
func (p *Pack) ContextOf(file string) string {
	if p == nil || p.Contexts == "" {
		return ""
	}
	pat := strings.Split(strings.Trim(p.Contexts, "/"), "/")
	segs := strings.Split(strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "./"), "/")
	// The context is a folder: the file is below it, never the folder's name.
	if len(segs) <= len(pat) {
		return ""
	}
	name := ""
	for i, s := range pat {
		switch {
		case s == "{context}":
			name = segs[i]
		case s != segs[i]:
			return ""
		}
	}
	return name
}

// Glob reports whether a slash path matches a pattern: path.Match for each
// name, and ** for any number of folders, none among them.
func Glob(pattern, file string) bool {
	return glob(strings.Split(pattern, "/"), strings.Split(file, "/"))
}

func glob(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if glob(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}

// IsTest reports whether a path is a test file of the language.
func (p *Pack) IsTest(file string) bool { return p != nil && matches(p.Tests, file) }

// IsSource reports whether a path is a source file of the language — code
// or test.
func (p *Pack) IsSource(file string) bool { return p != nil && matches(p.Sources, file) }

func matches(patterns []string, file string) bool {
	base := path.Base(strings.ReplaceAll(file, "\\", "/"))
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, base); ok {
			return true
		}
	}
	return false
}

// Command is how a sensor stage is checked. Report, when set, is the file
// the command writes its report to (SARIF, JUnit); empty reads stdout.
type Command struct {
	Run    string `yaml:"run"`
	Format string `yaml:"format"`
	Report string `yaml:"report,omitempty"`
}

// Load reads the pack gofi carries for a language; a language with none has
// no pack — its sensors come from the project's loop.sensors alone.
func Load(language string) (*Pack, error) {
	data, err := builtin.ReadFile("packs/" + language + ".yaml")
	if err != nil {
		return nil, nil
	}
	var p Pack
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("pack %s: %w", language, err)
	}
	if p.Schema != Schema {
		return nil, fmt.Errorf("pack %s: schema %q", language, p.Schema)
	}
	for stage, c := range p.Sensors {
		if !slices.Contains(formats, c.Format) {
			return nil, fmt.Errorf("pack %s: sensor %s: format %q (expected %s)", language, stage, c.Format, strings.Join(formats, ", "))
		}
	}
	return &p, nil
}

// Languages are the languages gofi carries a pack for.
func Languages() []string {
	entries, _ := builtin.ReadDir("packs")
	var out []string
	for _, e := range entries {
		out = append(out, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	sort.Strings(out)
	return out
}

// Expand puts the project's values into a command: {src} is the folder the
// language's code lives in.
func (c Command) Expand(src string) Command {
	if src == "" {
		src = "."
	}
	c.Run = strings.ReplaceAll(c.Run, "{src}", src)
	c.Report = strings.ReplaceAll(c.Report, "{src}", src)
	return c
}
