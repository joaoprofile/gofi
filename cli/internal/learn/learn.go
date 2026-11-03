// Package learn is the loop between runs (0002, §7): what a run had to fix
// — findings the audit raised, stages the sensors refused, tiers it raised
// — becomes lessons gofi-learn proposes for the project's knowledge. A
// lesson is only a proposal: the person accepts it, and only then is it
// written where every role reads it (knowledge/, the project's zone), or
// rejects it. The knowledge never changes on its own.
package learn

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Schema names the format of a run's lessons.
const Schema = "gofi-learn/v1"

// Dir is where the lessons of each run wait for the person, under the
// project root: one file a run.
const Dir = ".gofi/learn"

// Scopes are where a lesson may go in knowledge/: every role, or one.
var Scopes = []string{"shared", "spec", "eng", "qa", "ui", "ops"}

// The states of a lesson.
const (
	Pending  = "pendente"
	Accepted = "aceita"
	Rejected = "recusada"
)

// Lessons are what one run proposes.
type Lessons struct {
	Schema  string   `json:"schema"`
	Run     string   `json:"run"`
	Lessons []Lesson `json:"lessons"`
}

// Lesson is one thing a role should have known, written as the knowledge
// file it becomes.
type Lesson struct {
	// Scope is knowledge/<scope>/; Name the file, in kebab-case.
	Scope string `json:"scope"`
	Name  string `json:"name"`
	Title string `json:"title"`
	// Body is the rule and why, domain-neutral: it would serve, unchanged,
	// another project of the same SDK.
	Body      string   `json:"body"`
	Keywords  []string `json:"keywords"`
	Overrides []string `json:"overrides,omitempty"`
	// Evidence is what in the run taught it: the findings, the stages.
	Evidence string `json:"evidence"`
	Status   string `json:"status,omitempty"`
	// File is where an accepted lesson was written.
	File string `json:"file,omitempty"`
}

// Example is the shape of a run's lessons, as gofi-learn is told to write it.
var Example = fmt.Sprintf(`{"schema": %q, "lessons": [{"scope": "eng", "name": "<tema-em-kebab>", "title": "...", "body": "<a regra e o porquê, com placeholders>", "keywords": ["..."], "overrides": [], "evidence": "<o achado ou o estágio da execução que a ensinou>"}]}`, Schema)

var reName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Parse reads what gofi-learn wrote. A lesson gofi cannot place is an error,
// never one read as nothing.
func Parse(data []byte, run string) (*Lessons, error) {
	var l Lessons
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("lessons are not valid JSON of %s: %w", Schema, err)
	}
	if l.Schema != "" && l.Schema != Schema {
		return nil, fmt.Errorf("schema %q: this gofi reads %s", l.Schema, Schema)
	}
	l.Schema, l.Run = Schema, run
	if l.Lessons == nil {
		l.Lessons = []Lesson{}
	}
	var problems []string
	for i := range l.Lessons {
		ls := &l.Lessons[i]
		switch {
		case !slices.Contains(Scopes, ls.Scope):
			problems = append(problems, fmt.Sprintf("lesson %d: scope %q — %s", i+1, ls.Scope, strings.Join(Scopes, ", ")))
		case !reName.MatchString(ls.Name):
			problems = append(problems, fmt.Sprintf("lesson %d: name %q — kebab-case", i+1, ls.Name))
		case strings.TrimSpace(ls.Body) == "" || strings.TrimSpace(ls.Title) == "":
			problems = append(problems, fmt.Sprintf("lesson %d: title and body are required", i+1))
		}
		if ls.Status == "" {
			ls.Status = Pending
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return &l, nil
}

// Path is where a run's lessons wait.
func Path(root, run string) string {
	return filepath.Join(root, filepath.FromSlash(Dir), run+".json")
}

// Save keeps a run's lessons.
func Save(root string, l *Lessons) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	p := Path(root, l.Run)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

// Load reads a run's lessons.
func Load(root, run string) (*Lessons, error) {
	data, err := os.ReadFile(Path(root, run))
	if err != nil {
		return nil, err
	}
	return Parse(data, run)
}

// All are the lessons of every run, by run.
func All(root string) ([]*Lessons, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(Dir)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Lessons
	for _, e := range entries {
		if run, ok := strings.CutSuffix(e.Name(), ".json"); ok {
			l, err := Load(root, run)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", e.Name(), err)
			}
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Run < out[j].Run })
	return out, nil
}

// Accept writes a pending lesson into the project's knowledge and keeps
// that it was: knowledge/<scope>/<name>.md, never over a file the team
// already has.
func Accept(root string, l *Lessons, i int) (string, error) {
	if i < 0 || i >= len(l.Lessons) {
		return "", fmt.Errorf("run %s has no lesson %d", l.Run, i+1)
	}
	ls := &l.Lessons[i]
	if ls.Status != Pending {
		return "", fmt.Errorf("lesson %d of %s is %s", i+1, l.Run, ls.Status)
	}
	rel := layout.Knowledge().Path(ls.Scope, ls.Name+".md")
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("%s already exists: the team's file is not written over — edit it, or reject the lesson", rel)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(render(*ls)), 0o644); err != nil {
		return "", err
	}
	ls.Status, ls.File = Accepted, rel
	return rel, Save(root, l)
}

// Reject keeps that the person declined a lesson.
func Reject(root string, l *Lessons, i int) error {
	if i < 0 || i >= len(l.Lessons) {
		return fmt.Errorf("run %s has no lesson %d", l.Run, i+1)
	}
	if l.Lessons[i].Status != Pending {
		return fmt.Errorf("lesson %d of %s is %s", i+1, l.Run, l.Lessons[i].Status)
	}
	l.Lessons[i].Status = Rejected
	return Save(root, l)
}

// render is the knowledge file of a lesson: the frontmatter the corpus reads,
// the title, the body. The evidence stays in the lesson, not in the file:
// knowledge carries no provenance.
func render(ls Lesson) string {
	var b strings.Builder
	b.WriteString("---\n")
	if len(ls.Overrides) > 0 {
		fmt.Fprintf(&b, "overrides: [%s]\n", strings.Join(ls.Overrides, ", "))
	}
	fmt.Fprintf(&b, "keywords: [%s]\n---\n\n# %s\n\n%s\n", strings.Join(ls.Keywords, ", "), ls.Title, strings.TrimSpace(ls.Body))
	return b.String()
}
