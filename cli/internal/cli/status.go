package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scan"
	"github.com/joaoprofile/gofi/cli/internal/trace"
)

// StatusSchema names the format of gofi status --json.
const StatusSchema = "gofi-status/v1"

// ContextStatus is where a context is, from what is on disk: its memory, its
// PRD, its living spec and its approval, the changes open on it, its code and
// how much of the contract the code answers — and the next step.
type ContextStatus struct {
	Name   string `json:"name"`
	Memory string `json:"memory"` // the status its memory records; empty with no memory
	PRD    string `json:"prd,omitempty"`
	Spec   string `json:"spec,omitempty"`
	// SpecState is aprovada, vencida (edited after approval), proposta or
	// com-erros; empty with no living spec. Legacy is a format-2 spec that
	// gofi spec migrate turns into a living one.
	SpecState string   `json:"spec_state,omitempty"`
	Legacy    bool     `json:"legacy,omitempty"`
	Changes   []string `json:"changes,omitempty"`
	Sealed    []string `json:"sealed,omitempty"`
	Sources   int      `json:"sources"`
	Tests     int      `json:"tests"`
	// Requirements and Scenarios are how many the living spec has, and how
	// many the code answers: gofi:implements and gofi:tests.
	Requirements [2]int `json:"requirements"`
	Scenarios    [2]int `json:"scenarios"`
	Next         string `json:"next"`
	// Suggest is what is worth doing once nothing is missing: a PRD is the
	// person's choice — a technical context may need none.
	Suggest string `json:"suggest,omitempty"`
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [<context>...]",
		Short: i18n.T("cmd.status.short"),
		Long: `Where each context of the project is, read from the disk with no model — and
the next step of each one, in the order they are taken:

  memory      the context's memory, and the status it records
  PRD         prd/<context>/
  spec        the living spec, and whether it is approved, edited after its
              approval, still proposed, or not passing gofi spec check — or a
              format-2 spec that gofi spec migrate turns into one
  changes     the changes open on it, and the ones approved
  code        its sources and tests, as the language's pack lays them out
  trace       how many requirements the code implements and how many scenarios
              a test tests (gofi:implements, gofi:tests)

The contexts are the ones with a memory, a living spec, a PRD, or code the
folders name. /gofi-status reads this command.`,
		Example: `gofi status
gofi status pedidos
gofi status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			rows, err := projectStatus(root, cfg)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				rows = slices.DeleteFunc(rows, func(r ContextStatus) bool { return !slices.Contains(args, r.Name) })
				if len(rows) == 0 {
					return fmt.Errorf("%s", i18n.T("status.unknown", strings.Join(args, ", ")))
				}
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"schema": StatusSchema, "contexts": rows})
			}
			printStatus(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	return cmd
}

// projectStatus is every context of the project and where it is.
func projectStatus(root string, cfg *config.GofiConfig) ([]ContextStatus, error) {
	all, err := contract.Docs(root)
	if err != nil {
		return nil, err
	}
	rep, err := scan.Scan(root)
	if err != nil {
		return nil, err
	}
	byName := map[string]*ContextStatus{}
	get := func(name string) *ContextStatus {
		if byName[name] == nil {
			byName[name] = &ContextStatus{Name: name}
		}
		return byName[name]
	}

	// The memories, one file per context.
	if entries, err := os.ReadDir(layout.Contexts().Abs(root)); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			data, _ := os.ReadFile(layout.Contexts().Abs(root, e.Name()))
			fm, _ := frontmatterOf(string(data))
			get(name).Memory = strings.Trim(fm["status"], `"' `)
			if get(name).Memory == "" {
				get(name).Memory = "?"
			}
		}
	}
	// The PRDs and the format-2 specs, by folder.
	for _, dir := range []string{"prd", "specs"} {
		entries, _ := os.ReadDir(filepath.Join(root, dir))
		for _, e := range entries {
			if !e.IsDir() || e.Name() == contract.ChangesDir {
				continue
			}
			files, _ := filepath.Glob(filepath.Join(root, dir, e.Name(), "*.md"))
			for _, f := range files {
				base := filepath.Base(f)
				switch {
				case dir == "prd" && base != "INDEX.md":
					get(e.Name()).PRD = filepath.ToSlash(filepath.Join(dir, e.Name(), base))
				case dir == "specs" && strings.HasPrefix(base, "sdd-"):
					get(e.Name()).Legacy = true
				}
			}
		}
	}
	// The living specs, their approval and whether they pass the check.
	placeholders, _ := contract.Placeholders(root)
	for _, d := range all {
		if d.Kind != contract.KindSpec || d.Change != "" || d.Context == "" {
			continue
		}
		c := get(d.Context)
		c.Spec = d.Path
		switch state, _ := contract.SealOf(d); {
		case contract.Errors(contract.Check(root, all, d, placeholders)) > 0:
			c.SpecState = "com-erros"
		case state == contract.Sealed:
			c.SpecState = "aprovada"
		case state == contract.Stale:
			c.SpecState = "vencida"
		default:
			c.SpecState = "proposta"
		}
		var reqs, scens int
		for _, r := range contract.Requirements(d) {
			reqs++
			scens += len(r.Scenarios)
		}
		c.Requirements[0], c.Scenarios[0] = reqs, scens
	}
	for _, ch := range contract.Changes(all) {
		for _, ctx := range ch.Contexts {
			c := get(ctx)
			c.Changes = append(c.Changes, ch.ID)
			if ch.Sealed {
				c.Sealed = append(c.Sealed, ch.ID)
			}
		}
	}
	for _, sc := range rep.Contexts {
		c := get(sc.Name)
		c.Sources, c.Tests = sc.Sources, sc.Tests
	}

	// How much of each living spec the code answers.
	if lp, err := projectPack(cfg); err == nil {
		src := ""
		if cfg.Backend != nil {
			src = cfg.Backend.Path
		}
		if marks, err := trace.Scan(root, src, lp); err == nil {
			var sources []trace.Source
			for _, d := range all {
				if d.Kind == contract.KindSpec && d.Change == "" && d.Context != "" {
					sources = append(sources, trace.Source{Context: d.Context, Doc: d.Path, Requirements: contract.Requirements(d)})
				}
			}
			for _, row := range trace.Matrix(sources, marks) {
				c := get(row.Context)
				switch {
				case row.Scenario && !row.Gap():
					c.Scenarios[1]++
				case !row.Scenario && !row.Gap():
					c.Requirements[1]++
				}
			}
		}
	}

	var out []ContextStatus
	for _, c := range byName {
		c.Next = nextStep(*c)
		if c.Next == "" && c.PRD == "" {
			c.Suggest = "/gofi-pd " + c.Name
		}
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// nextStep is the first thing a context lacks, in the order it is done: a
// memory, a contract, an approved one, its changes delivered, its code tied
// to it.
func nextStep(c ContextStatus) string {
	unsealed := slices.DeleteFunc(slices.Clone(c.Changes), func(id string) bool { return slices.Contains(c.Sealed, id) })
	switch {
	case c.Memory == "" && c.Sources > 0:
		return "gofi context adopt " + c.Name
	case c.Spec == "" && c.Legacy:
		return "gofi spec migrate " + c.Name
	case c.Spec == "" && len(c.Changes) == 0:
		return "/gofi-spec " + c.Name
	case c.SpecState == "com-erros":
		return "gofi spec check " + c.Name
	case c.SpecState == "proposta" || c.SpecState == "vencida":
		return "gofi spec approve " + c.Name
	case len(unsealed) > 0:
		return "gofi review " + unsealed[0] + " · gofi change approve " + unsealed[0]
	case len(c.Sealed) > 0:
		return "gofi run " + c.Sealed[0]
	case c.Requirements[1] < c.Requirements[0] || c.Scenarios[1] < c.Scenarios[0]:
		return "gofi trace " + c.Name
	}
	return ""
}

func printStatus(w io.Writer, rows []ContextStatus) {
	if len(rows) == 0 {
		fmt.Fprintln(w, i18n.T("status.none"))
		return
	}
	mark := func(ok bool) string {
		if ok {
			return "✓"
		}
		return "✗"
	}
	done := 0
	for _, c := range rows {
		memory := c.Memory
		if memory == "" {
			memory = i18n.T("status.no_memory")
		}
		fmt.Fprintf(w, "%s — %s\n", c.Name, memory)
		spec := mark(c.Spec != "")
		if c.SpecState != "" {
			spec += " " + c.SpecState
		} else if c.Legacy {
			spec += " " + i18n.T("status.legacy")
		}
		parts := []string{"PRD " + mark(c.PRD != ""), "spec " + spec}
		if len(c.Changes) > 0 {
			parts = append(parts, i18n.T("status.changes", len(c.Changes), len(c.Sealed)))
		}
		if c.Sources+c.Tests > 0 {
			parts = append(parts, i18n.T("status.code", c.Sources, c.Tests))
		}
		if c.Requirements[0] > 0 {
			parts = append(parts, i18n.T("status.trace", c.Requirements[1], c.Requirements[0], c.Scenarios[1], c.Scenarios[0]))
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(parts, " · "))
		if c.Next == "" {
			line := i18n.T("status.done")
			if c.Suggest != "" {
				line += " · " + i18n.T("status.suggest", c.Suggest)
			}
			fmt.Fprintf(w, "  %s\n\n", line)
			done++
			continue
		}
		fmt.Fprintf(w, "  → %s\n\n", c.Next)
	}
	fmt.Fprintln(w, i18n.T("status.summary", done, len(rows)))
}
