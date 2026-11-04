package bench

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// AuditCase is code already written with defects planted in it, and the
// change it claims to realize: what an audit is measured on (0002, M7 —
// recall of the lenses against the general lens alone, on the same code).
type AuditCase struct {
	Name    string   `yaml:"-"`
	Change  string   `yaml:"change"`
	Defects []Defect `yaml:"defects"`
	lang    string
}

// Defect is one planted defect: a finding finds it when it is in its file,
// within its lines, and says what it is about — or, with no file, names its
// requirement and says it.
type Defect struct {
	ID       string   `yaml:"id" json:"id"`
	What     string   `yaml:"what" json:"what"`
	File     string   `yaml:"file,omitempty" json:"file,omitempty"`
	Lines    [2]int   `yaml:"lines,omitempty" json:"lines,omitempty"`
	Rule     string   `yaml:"rule,omitempty" json:"rule,omitempty"`
	Keywords []string `yaml:"keywords" json:"keywords"`
}

// AuditCases are a language's audit cases.
func AuditCases(language string) ([]*AuditCase, error) {
	dir := path.Join("suites", language, "audit")
	entries, err := suites.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var out []*AuditCase
	for _, e := range entries {
		data, err := suites.ReadFile(path.Join(dir, e.Name(), "defects.yaml"))
		if err != nil {
			return nil, fmt.Errorf("audit case %s: %w", e.Name(), err)
		}
		c := &AuditCase{Name: e.Name(), lang: language}
		if err := yaml.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("audit case %s: %w", e.Name(), err)
		}
		out = append(out, c)
	}
	return out, nil
}

// Write puts the case in a project: its change in specs/changes/, its code
// in the backend folder src.
func (c *AuditCase) Write(root, src string) error {
	base := path.Join("suites", c.lang, "audit", c.Name)
	copyDir := func(from, to string) error {
		return fs.WalkDir(suites, from, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := suites.ReadFile(p)
			if err != nil {
				return err
			}
			// Code is kept as .tmpl, so gofi's own module never builds it.
			dst := filepath.Join(to, filepath.FromSlash(strings.TrimSuffix(strings.TrimPrefix(p, from+"/"), ".tmpl")))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			return os.WriteFile(dst, data, 0o644)
		})
	}
	if err := copyDir(path.Join(base, "change"), filepath.Join(root, "specs", "changes", c.Change)); err != nil {
		return err
	}
	return copyDir(path.Join(base, "code"), filepath.Join(root, filepath.FromSlash(src)))
}

// Score is how an audit did on a case.
type Score struct {
	Case  string   `json:"case"`
	Mode  string   `json:"mode"`
	Found []string `json:"found"`
	// Missed are the defects no finding found; Extra the findings that
	// found none — a defect not planted, or noise.
	Missed  []string `json:"missed"`
	Extra   int      `json:"extra"`
	CostUSD float64  `json:"cost_usd"`
	Turns   int      `json:"turns"`
}

// Recall is the share of the planted defects found.
func (s Score) Recall() float64 {
	n := len(s.Found) + len(s.Missed)
	if n == 0 {
		return 0
	}
	return float64(len(s.Found)) / float64(n)
}

// Judge scores a verdict against the case's defects; src is the backend
// folder its files are under.
func (c *AuditCase) Judge(v *verdict.Verdict, src string) Score {
	s := Score{Case: c.Name, Found: []string{}, Missed: []string{}}
	matched := map[int]bool{}
	for _, d := range c.Defects {
		hit := false
		for i, f := range v.Findings {
			if d.finds(f, src) {
				hit, matched[i] = true, true
			}
		}
		if hit {
			s.Found = append(s.Found, d.ID)
		} else {
			s.Missed = append(s.Missed, d.ID)
		}
	}
	for i, f := range v.Findings {
		if !matched[i] && f.Severity != "suggestion" {
			s.Extra++
		}
	}
	return s
}

func (d Defect) finds(f verdict.Finding, src string) bool {
	text := strings.ToLower(f.Rule + " " + f.Message + " " + f.Evidence)
	said := len(d.Keywords) == 0 || slices.ContainsFunc(d.Keywords, func(k string) bool { return strings.Contains(text, strings.ToLower(k)) })
	if !said {
		return false
	}
	if d.File == "" {
		return d.Rule != "" && strings.Contains(strings.ToUpper(f.Rule+" "+f.Message), strings.ToUpper(d.Rule))
	}
	file := strings.ReplaceAll(d.File, "{src}", src)
	if strings.TrimPrefix(f.File, "./") != file {
		return false
	}
	// A finding with no line is placed by its file and what it says.
	return f.Line == 0 || f.Line >= d.Lines[0]-2 && f.Line <= d.Lines[1]+2
}

// AuditReport is the comparison, for a person.
func AuditReport(id string, scores []Score) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark de auditoria `%s`\n\nA mesma implementação, com defeitos plantados, auditada em lentes e pela lente geral sozinha.\n\n", id)
	b.WriteString("| Caso | Auditoria | Recall | Achou | Perdeu | Achados a mais | Turnos | US$ |\n|---|---|---|---|---|---|---|---|\n")
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].Case < scores[j].Case })
	for _, s := range scores {
		fmt.Fprintf(&b, "| %s | %s | %.0f%% | %s | %s | %d | %d | %.2f |\n", s.Case, s.Mode, 100*s.Recall(),
			strings.Join(s.Found, " "), strings.Join(s.Missed, " "), s.Extra, s.Turns, s.CostUSD)
	}
	return b.String()
}
