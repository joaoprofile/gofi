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
	FormatExit       = "exit"         // only the exit code says anything
)

var formats = []string{FormatLines, FormatGoTestJSON, FormatSARIF, FormatExit}

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
}

// IsTest reports whether a path is a test file of the language.
func (p *Pack) IsTest(file string) bool {
	if p == nil {
		return false
	}
	base := path.Base(strings.ReplaceAll(file, "\\", "/"))
	for _, pat := range p.Tests {
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
