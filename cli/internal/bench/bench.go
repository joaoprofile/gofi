// Package bench measures the loop (0002, §10): a fixed suite of changes,
// each built to exercise one thing — a change that should pass at once, a
// contract with a gap, a trap that tends to need rework, a change of two
// layers —, delivered on a copy of the project that never lands, and what
// each run did read back from its folder: accepted or not, the rounds, the
// cost by role, the lessons it proposed, the caveats of its contract.
// Nothing in it is a model's word.
package bench

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joaoprofile/gofi/cli/internal/docs"

	"github.com/joaoprofile/gofi/cli/internal/learn"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

//go:embed all:suites
var suites embed.FS

// Schema names the format of a suite and of a benchmark's result.
const Schema = "gofi-bench/v1"

// Dir is where a project keeps its benchmarks' results.
const Dir = ".gofi/bench"

// Suite is the changes a language's benchmark delivers.
type Suite struct {
	Schema   string   `yaml:"schema"`
	Language string   `yaml:"language"`
	Changes  []Change `yaml:"changes"`
}

// Change is one change of a suite, and what it is built to show.
type Change struct {
	ID     string `yaml:"id" json:"id"`
	Kind   string `yaml:"kind" json:"kind"`
	Expect string `yaml:"expect" json:"expect"`
}

// Load is the suite of a language; ok is false when gofi has none.
func Load(language string) (*Suite, bool, error) {
	data, err := suites.ReadFile(path.Join("suites", language, "suite.yaml"))
	if err != nil {
		return nil, false, nil
	}
	var s Suite
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, true, fmt.Errorf("suite %s: %w", language, err)
	}
	return &s, true, nil
}

// changeHome is where a suite's change goes in a project: the folder of the
// first context its proposal names, as gofi change new puts it.
func changeHome(root, src, id string) (string, error) {
	data, err := suites.ReadFile(path.Join(src, "proposal.md"))
	if err != nil {
		return "", err
	}
	fm, _ := docs.Parse(string(data))
	contexts := fm.List("contextos")
	if len(contexts) == 0 {
		return "", fmt.Errorf("%s: a proposta não diz os contextos", src)
	}
	return filepath.Join(root, "specs", contexts[0], "changes", id), nil
}

// Write puts the suite's changes in a project, each in its context's folder.
func (s *Suite) Write(root string) error {
	for _, c := range s.Changes {
		src := path.Join("suites", s.Language, c.ID)
		home, err := changeHome(root, src, c.ID)
		if err != nil {
			return err
		}
		err = fs.WalkDir(suites, src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := suites.ReadFile(p)
			if err != nil {
				return err
			}
			dst := filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(p, src+"/")))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			return os.WriteFile(dst, data, 0o644)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Result is a benchmark: each change's run, and the totals.
type Result struct {
	Schema   string    `json:"schema"`
	ID       string    `json:"id"`
	Language string    `json:"language"`
	Runs     []RunStat `json:"runs"`
	// CostUSD is what the runs spent; ByRole, by role.
	CostUSD float64            `json:"cost_usd"`
	ByRole  map[string]float64 `json:"by_role"`
	// Accepted counts the changes accepted; FirstPass those accepted with
	// no rework.
	Accepted  int `json:"accepted"`
	FirstPass int `json:"first_pass"`
}

// RunStat is one change's run as it ended.
type RunStat struct {
	Change
	Run      string  `json:"run,omitempty"`
	Outcome  string  `json:"outcome"`
	Rounds   int     `json:"rounds"`
	CostUSD  float64 `json:"cost_usd"`
	Turns    int     `json:"turns"`
	Findings int     `json:"findings"`
	Notes    int     `json:"notes"`
	Lessons  int     `json:"lessons"`
	// Roles are the roles that implemented, in the order they ran.
	Roles []string `json:"roles"`
}

// Collect reads what each change of the suite's run did, from the runs of
// the project at dir.
func Collect(dir, id string, s *Suite) (*Result, error) {
	res := &Result{Schema: Schema, ID: id, Language: s.Language, ByRole: map[string]float64{}}
	infos, err := loop.List(dir)
	if err != nil {
		return nil, err
	}
	latest := map[string]string{}
	for _, i := range infos {
		if st := i.State; st != nil && st.Run > latest[st.Change] {
			latest[st.Change] = st.Run
		}
	}
	for _, c := range s.Changes {
		rs := RunStat{Change: c, Outcome: "não rodou", Roles: []string{}}
		if run := latest[c.ID]; run != "" {
			st, err := loop.LoadState(dir, run)
			if err != nil {
				return nil, err
			}
			rs.Run, rs.Outcome, rs.Rounds, rs.CostUSD = run, st.Outcome, st.Rounds, st.CostUSD
			rs.Findings, rs.Notes = len(st.Seen), len(st.Notes)
			agents, _ := loop.Agents(dir, run)
			rs.Turns = len(agents)
			for _, a := range agents {
				res.ByRole[a.Role] += a.CostUSD
				if a.Node == "implement" && !contains(rs.Roles, a.Role) {
					rs.Roles = append(rs.Roles, a.Role)
				}
			}
			if l, err := learn.Load(dir, run); err == nil {
				rs.Lessons = len(l.Lessons)
			}
			if st.Outcome == "accepted" {
				res.Accepted++
				if st.Rounds == 0 {
					res.FirstPass++
				}
			}
		}
		res.CostUSD += rs.CostUSD
		res.Runs = append(res.Runs, rs)
	}
	return res, nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Save keeps a benchmark's result in a project, as JSON and as the report a
// person reads.
func (r *Result) Save(root string) (string, error) {
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, r.ID+".json"), append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	p := filepath.Join(dir, r.ID+".md")
	return p, os.WriteFile(p, []byte(r.Report()), 0o644)
}

// Report is the result for a person.
func (r *Result) Report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark `%s` — %s\n\n", r.ID, r.Language)
	fmt.Fprintf(&b, "%d de %d aceitas · %d na primeira volta · US$ %.2f\n\n", r.Accepted, len(r.Runs), r.FirstPass, r.CostUSD)
	b.WriteString("| Mudança | Mede | Resultado | Voltas | Achados | Ressalvas | Lições | Implementaram | Turnos | US$ |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, s := range r.Runs {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %d | %s | %d | %.2f |\n",
			s.ID, s.Kind, s.Outcome, s.Rounds, s.Findings, s.Notes, s.Lessons, strings.Join(s.Roles, ", "), s.Turns, s.CostUSD)
	}
	b.WriteString("\n## O que cada mudança devia mostrar\n\n")
	for _, s := range r.Runs {
		fmt.Fprintf(&b, "- **%s** — %s\n", s.ID, s.Expect)
	}
	roles := make([]string, 0, len(r.ByRole))
	for role := range r.ByRole {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return r.ByRole[roles[i]] > r.ByRole[roles[j]] })
	b.WriteString("\n## Custo por papel\n\n| Papel | US$ |\n|---|---|\n")
	for _, role := range roles {
		fmt.Fprintf(&b, "| %s | %.2f |\n", role, r.ByRole[role])
	}
	return b.String()
}
