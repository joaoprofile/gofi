package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/scan"
)

func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: i18n.T("cmd.scan.short"),
		Long: `Read a repository — a gofi project or not — and say what it already holds of
one, and what is next. It writes nothing and asks no model.

  code        the backend, web and mobile apps; the backend's sources and tests,
              and the contexts its folders name, as the language's pack lays
              them out
  documents   what reads as a PRD (problem, goals, personas, scope), as a spec
              (requirements, rules, acceptance criteria), as a decision record,
              as operations — each with the words that say so
  scenarios   Gherkin .feature files: acceptance already written
  contracts   OpenAPI, protobuf and GraphQL APIs
  tables      what the schema and the migrations create, and the context that
              names each one
  gofi        .gofi.yaml, AGENTS.md, the skills, the specs and PRDs, the graph

The next steps say which command or skill turns each into gofi's own: gofi
init adopts the repository, /gofi-pd and /gofi-spec rewrite what is there into
PRDs and living specs — the originals are never moved.`,
		Example: `gofi scan
gofi scan ../legacy-service
gofi scan --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			if info, err := os.Stat(root); err != nil || !info.IsDir() {
				return fmt.Errorf("%s", i18n.T("scan.no_dir", root))
			}
			r, err := scan.Scan(root)
			if err != nil {
				return err
			}
			steps := scanSteps(r)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				out := struct {
					*scan.Report
					Next []string `json:"next"`
				}{r, steps}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			printScan(cmd.OutOrStdout(), r, steps)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	return cmd
}

// maxListed is how many of a kind the report names; the JSON has them all.
const maxListed = 8

// printScan writes what the scan found, for a person.
func printScan(w io.Writer, r *scan.Report, steps []string) {
	section := func(title string) { fmt.Fprintf(w, "\n%s\n", title) }
	fmt.Fprintf(w, "%s\n", i18n.T("scan.title", r.Root))

	section(i18n.T("scan.gofi"))
	if r.Gofi.Config {
		fmt.Fprintf(w, "  %s\n", i18n.T("scan.gofi.yes", r.Gofi.Specs, r.Gofi.Contract, r.Gofi.PRDs))
	} else {
		fmt.Fprintf(w, "  %s\n", i18n.T("scan.gofi.no"))
	}

	section(i18n.T("scan.code"))
	for _, s := range []struct {
		label string
		s     *scan.Surface
	}{{"backend", r.Backend}, {"web", r.Web}, {"mobile", r.Mobile}} {
		if s.s == nil {
			continue
		}
		what := strings.TrimSpace(s.s.Language + " " + s.s.Framework)
		if s.s.Module != "" {
			what += " · " + s.s.Module
		}
		fmt.Fprintf(w, "  %-8s %s (%s)\n", s.label, s.s.Path, what)
	}
	if r.Backend == nil && r.Web == nil && r.Mobile == nil {
		fmt.Fprintf(w, "  %s\n", i18n.T("scan.code.none"))
	}
	if r.Backend != nil {
		fmt.Fprintf(w, "  %s\n", i18n.T("scan.code.counts", r.Sources, r.Tests))
	}

	if len(r.Contexts) > 0 {
		section(i18n.T("scan.contexts", len(r.Contexts)))
		for i, c := range r.Contexts {
			if i == maxListed {
				fmt.Fprintf(w, "  … +%d\n", len(r.Contexts)-maxListed)
				break
			}
			line := fmt.Sprintf("  %-20s %s", c.Name, i18n.T("scan.context.line", c.Sources, c.Tests))
			if len(c.Tables) > 0 {
				line += " · " + i18n.T("scan.context.tables", strings.Join(c.Tables, ", "))
			}
			if len(c.Documents) > 0 {
				line += " · " + i18n.T("scan.context.docs", len(c.Documents))
			}
			fmt.Fprintln(w, line)
		}
	}

	kinds := []struct{ kind, key string }{
		{scan.KindPRD, "scan.docs.prd"}, {scan.KindSpec, "scan.docs.spec"},
		{scan.KindAPI, "scan.docs.api"}, {scan.KindDecision, "scan.docs.decision"}, {scan.KindOps, "scan.docs.ops"},
	}
	for _, k := range kinds {
		docs := r.Of(k.kind)
		if len(docs) == 0 {
			continue
		}
		section(i18n.T(k.key, len(docs)))
		for i, d := range docs {
			if i == maxListed {
				fmt.Fprintf(w, "  … +%d\n", len(docs)-maxListed)
				break
			}
			fmt.Fprintf(w, "  %s — %s\n", d.Path, strings.Join(d.Evidence, ", "))
		}
	}
	if n := len(r.Of(scan.KindOther)) + len(r.Of(scan.KindReadme)) + len(r.Of(scan.KindChangelog)); n > 0 {
		fmt.Fprintf(w, "\n%s\n", i18n.T("scan.docs.other", n))
	}

	if len(r.Features) > 0 {
		scenarios := 0
		for _, f := range r.Features {
			scenarios += f.Scenarios
		}
		section(i18n.T("scan.features", len(r.Features), scenarios))
		for i, f := range r.Features {
			if i == maxListed {
				fmt.Fprintf(w, "  … +%d\n", len(r.Features)-maxListed)
				break
			}
			fmt.Fprintf(w, "  %s — %s (%d)\n", f.Path, f.Title, f.Scenarios)
		}
	}
	if len(r.APIs) > 0 {
		section(i18n.T("scan.apis", len(r.APIs)))
		for _, a := range r.APIs {
			fmt.Fprintf(w, "  %s — %s%s · %d\n", a.Path, a.Format, map[bool]string{true: " " + a.Title, false: ""}[a.Title != ""], a.Operations)
		}
	}
	if len(r.Tables) > 0 {
		names := make([]string, 0, len(r.Tables))
		for _, t := range r.Tables {
			names = append(names, t.Name)
		}
		section(i18n.T("scan.tables", len(r.Tables)))
		if len(names) > 3*maxListed {
			names = append(names[:3*maxListed], fmt.Sprintf("… +%d", len(r.Tables)-3*maxListed))
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(names, ", "))
	}

	section(i18n.T("scan.next"))
	for i, s := range steps {
		fmt.Fprintf(w, "  %d. %s\n", i+1, s)
	}
}

// scanSteps are what is next, in the order it is done: adopt, then the
// contract from what is there, then the code tied to it.
func scanSteps(r *scan.Report) []string {
	var out []string
	add := func(key string, args ...any) { out = append(out, i18n.T(key, args...)) }
	if !r.Gofi.Config {
		add("scan.step.init")
	} else if !r.Gofi.Graph {
		add("scan.step.index")
	}
	if r.Gofi.ClaudeDir {
		add("scan.step.agents")
	}
	if r.Gofi.ClaudeRoot {
		add("scan.step.claude_root")
	}
	if r.Backend != nil && len(r.Contexts) == 0 {
		add("scan.step.no_contexts")
	}
	if n := len(r.Of(scan.KindPRD)); n > 0 {
		add("scan.step.prd", n)
	}
	scenarios := 0
	for _, f := range r.Features {
		scenarios += f.Scenarios
	}
	if n := len(r.Of(scan.KindSpec)); n > 0 || scenarios > 0 {
		add("scan.step.spec", n, scenarios)
	}
	if r.Gofi.Specs > 0 && r.Gofi.Contract == 0 {
		add("scan.step.spec_migrate")
	}
	if n := len(r.APIs) + len(r.Of(scan.KindAPI)); n > 0 {
		add("scan.step.apis", n)
	}
	if n := len(r.Of(scan.KindDecision)); n > 0 {
		add("scan.step.decisions", n)
	}
	// Before the living specs, the tests are what the scenarios start from.
	if r.Tests > 0 && r.Gofi.Contract == 0 {
		add("scan.step.tests", r.Tests)
	}
	if len(out) == 0 {
		add("scan.step.none")
	}
	return out
}
