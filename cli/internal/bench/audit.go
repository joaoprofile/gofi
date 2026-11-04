package bench

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
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

// Defect is one planted defect: a finding finds it when it says what it is
// about and is in its file, within its lines — or names that place in what
// it says, or, with no file, names the defect's requirement: Rule.
type Defect struct {
	ID    string `yaml:"id" json:"id"`
	What  string `yaml:"what" json:"what"`
	File  string `yaml:"file,omitempty" json:"file,omitempty"`
	Lines [2]int `yaml:"lines,omitempty" json:"lines,omitempty"`
	// At places the defect by the text of its line, instead of Lines: the
	// first line of File holding it, and Span lines after it.
	At   string `yaml:"at,omitempty" json:"at,omitempty"`
	Span int    `yaml:"span,omitempty" json:"span,omitempty"`
	// Rule is the requirement the defect breaks — REQ-01 —, or, for a
	// defect with no file, the prefix of what a finding of it names.
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
// in the backend folder src — {module} in it the module its go.mod names —,
// and places each defect given by the text of its line.
func (c *AuditCase) Write(root, src string) error {
	module := ""
	if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src), "go.mod")); err == nil {
		for _, ln := range strings.Split(string(data), "\n") {
			if m, ok := strings.CutPrefix(strings.TrimSpace(ln), "module "); ok {
				module = strings.TrimSpace(m)
			}
		}
	}
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
			if strings.HasSuffix(p, ".tmpl") {
				data = []byte(strings.ReplaceAll(string(data), "{module}", module))
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
	if err := copyDir(path.Join(base, "code"), filepath.Join(root, filepath.FromSlash(src))); err != nil {
		return err
	}
	return c.place(root, src)
}

// place resolves the defects given by the text of their line.
func (c *AuditCase) place(root, src string) error {
	for i := range c.Defects {
		d := &c.Defects[i]
		if d.At == "" || d.File == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(d.File, "{src}", src))))
		if err != nil {
			return fmt.Errorf("defect %s: %w", d.ID, err)
		}
		found := false
		for n, ln := range strings.Split(string(data), "\n") {
			if strings.Contains(ln, d.At) {
				d.Lines, found = [2]int{n + 1, n + 1 + d.Span}, true
				break
			}
		}
		if !found {
			return fmt.Errorf("defect %s: %q is in no line of %s", d.ID, d.At, d.File)
		}
	}
	return nil
}

// Score is how an audit did on a case.
type Score struct {
	Case  string   `json:"case"`
	Mode  string   `json:"mode"`
	Found []string `json:"found"`
	// Missed are the defects no finding found; Extra the findings that
	// found none — a defect not planted, or noise.
	Missed []string `json:"missed"`
	Extra  int      `json:"extra"`
	// Extras are those findings, as the audit wrote them: what to read to
	// tell a defect nobody planted from noise.
	Extras  []string `json:"extras,omitempty"`
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
			s.Extras = append(s.Extras, fmt.Sprintf("[%s] %s — %s", f.Severity, f.Rule, f.Message))
		}
	}
	return s
}

func (d Defect) finds(f verdict.Finding, src string) bool {
	text := fold(f.Rule + " " + f.Message + " " + f.Evidence)
	said := len(d.Keywords) == 0 || slices.ContainsFunc(d.Keywords, func(k string) bool { return strings.Contains(text, fold(k)) })
	if !said {
		return false
	}
	if d.File == "" {
		return d.Rule != "" && strings.Contains(strings.ToUpper(f.Rule+" "+f.Message), strings.ToUpper(d.Rule))
	}
	file := strings.ReplaceAll(d.File, "{src}", src)
	// A finding of the contract, with no file, finds it by the requirement
	// it breaks — the requirement, not one of its scenarios: "REQ-03.2 has
	// no test" is another defect.
	if f.File == "" {
		return d.Rule != "" && namesRequirement(f.Rule, d.Rule)
	}
	// In its file: a finding with no line, on its lines, naming one of them
	// — placed on the function, it says "linha 27" —, or naming the
	// requirement it breaks.
	if strings.TrimPrefix(f.File, "./") == file && (f.Line == 0 || d.near(f.Line) || d.namedLine(f.Message+" "+f.Evidence) || d.Rule != "" && namesRequirement(f.Rule, d.Rule)) {
		return true
	}
	// One kept on another file finds it by the place it names: "the handler
	// answers 500 (x_handler.go:33)".
	for _, m := range rePlace.FindAllStringSubmatch(f.Message+" "+f.Evidence, -1) {
		n, _ := strconv.Atoi(m[2])
		if (m[1] == file || strings.HasSuffix(file, "/"+m[1])) && d.near(n) {
			return true
		}
	}
	return false
}

// unaccent folds what an audit writes and a case's keywords alike:
// "repositor" says "repositório", "atômic" says "atomic".
var unaccent = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c")

func fold(s string) string { return unaccent.Replace(strings.ToLower(s)) }

// rePlace is a place a finding names in its text: file.ext:line.
var rePlace = regexp.MustCompile(`([\w./-]+\.\w+):(\d+)`)

func (d Defect) near(line int) bool {
	return line >= d.Lines[0]-2 && line <= d.Lines[1]+2
}

// reLine is a line a finding names in its text: linha 27, linhas 32-34,
// L29, line 12.
var reLine = regexp.MustCompile(`(?i)\b(?:linhas?|lines?|l)\s?(\d+)(?:\s?[-–]\s?(\d+))?\b`)

// namedLine reports whether a text names a line of the defect.
func (d Defect) namedLine(text string) bool {
	for _, m := range reLine.FindAllStringSubmatch(text, -1) {
		from, _ := strconv.Atoi(m[1])
		to := from
		if m[2] != "" {
			to, _ = strconv.Atoi(m[2])
		}
		if from <= d.Lines[1]+2 && to >= d.Lines[0]-2 {
			return true
		}
	}
	return false
}

// namesRequirement reports whether a finding's rule names the requirement
// itself: REQ-01, not REQ-01.2 nor REQ-012.
func namesRequirement(rule, req string) bool {
	rule, req = strings.ToUpper(rule), strings.ToUpper(req)
	for i := strings.Index(rule, req); i >= 0; {
		rest := rule[i+len(req):]
		if rest == "" || rest[0] != '.' && (rest[0] < '0' || rest[0] > '9') {
			return true
		}
		j := strings.Index(rest, req)
		if j < 0 {
			break
		}
		i += len(req) + j
	}
	return false
}

// AuditReport is the comparison, for a person.
func AuditReport(id string, scores []Score) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark de auditoria `%s`\n\nA mesma implementação, com defeitos plantados, auditada em lentes e pela lente geral sozinha.\n\n", id)
	b.WriteString("| Caso | Auditoria | Recall | Achou | Perdeu | Achados a mais | Turnos | US$ |\n|---|---|---|---|---|---|---|---|\n")
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].Case != scores[j].Case {
			return scores[i].Case < scores[j].Case
		}
		return scores[i].Mode < scores[j].Mode
	})
	for _, s := range scores {
		fmt.Fprintf(&b, "| %s | %s | %.0f%% | %s | %s | %d | %d | %.2f |\n", s.Case, s.Mode, 100*s.Recall(),
			strings.Join(s.Found, " "), strings.Join(s.Missed, " "), s.Extra, s.Turns, s.CostUSD)
	}
	b.WriteString("\n## Achados além dos plantados\n\nUm defeito real que ninguém plantou, ou ruído: leia para decidir.\n")
	for _, s := range scores {
		if len(s.Extras) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s · %s\n\n", s.Case, s.Mode)
		for _, e := range s.Extras {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	return b.String()
}
