package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/graph/workspace"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// statusSchema versions the JSON `gofi index status --json` writes.
const statusSchema = "gofi.index-status/v1"

// IndexStatus is what `gofi index status` reports: each target's last build
// and whether it still matches the sources.
type IndexStatus struct {
	Schema string       `json:"schema"`
	Code   TargetStatus `json:"code"`
	Docs   TargetStatus `json:"docs"`
	// Legacy is set while the project still has an index in the old layout.
	Legacy bool `json:"legacy,omitempty"`
}

// TargetStatus is one target of the index.
type TargetStatus struct {
	Dir    string                 `json:"dir"`
	State  workspace.Freshness    `json:"state"`
	Built  *layout.Build          `json:"built,omitempty"`
	Scopes []workspace.ScopeState `json:"scopes,omitempty"`
}

// Current reports whether nothing needs rebuilding. A target whose freshness
// cannot be told is not held against it: that is an extractor's limit, not a
// sign the index is behind.
func (s IndexStatus) Current() bool {
	ok := func(f workspace.Freshness) bool { return f == workspace.Fresh || f == workspace.Unknown }
	return ok(s.Code.State) && ok(s.Docs.State) && !s.Legacy
}

func newIndexStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: i18n.T("cmd.index.status.short"),
		Long: `Report whether the index still matches the project, without building
anything: when each target was last built, and for every code scope whether a
source file was added, removed or edited since.

A scope read by an external extractor reports unknown — only the extractor
knows which files its language compiles. With --check, exits non-zero when a
target is stale or was never built, for CI and git hooks.`,
		Example: `gofi index status
gofi index status --check
gofi index status --json`,
		Args: cobra.NoArgs,
		RunE: runIndexStatus,
	}
	cmd.Flags().Bool("check", false, i18n.T("cmd.index.flag.check"))
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	return cmd
}

func runIndexStatus(cmd *cobra.Command, _ []string) error {
	found, err := findProjectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(found, config.FileName))
	if err != nil {
		return err
	}
	root := declaredRoot(cfg, found)
	st, err := indexStatus(cfg, root)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		if err := writeJSON(out, st); err != nil {
			return err
		}
	} else {
		printIndexStatus(out, st)
	}
	if check, _ := cmd.Flags().GetBool("check"); check && !st.Current() {
		return errors.New(i18n.T("index.status.behind"))
	}
	return nil
}

// indexStatus compares both targets with the project. The code check repeats
// the scan options the last build recorded; without a record it uses the ones
// the project declares, which is what a build with no flags would have used.
func indexStatus(cfg *config.GofiConfig, root string) (IndexStatus, error) {
	m, err := layout.LoadManifest(root)
	if err != nil {
		return IndexStatus{}, err
	}
	st := IndexStatus{
		Schema: statusSchema,
		Legacy: layout.HasLegacy(root),
		Code:   TargetStatus{Dir: layout.CodeDir, Built: m.Code},
		Docs:   TargetStatus{Dir: layout.DocsDir, Built: m.Docs},
	}

	if graphEnabled(cfg) {
		opt := graphOptions(cfg, root)
		if b := m.Code; b != nil && b.Options != nil {
			opt.WithTests, opt.Exclude, opt.MaxFileKB = b.Options.WithTests, b.Options.Exclude, b.Options.MaxFileKB
		}
		if st.Code.Scopes, err = workspace.Check(opt); err != nil {
			return st, err
		}
		st.Code.State = overall(st.Code.Scopes)
	} else {
		st.Code.State = workspace.Fresh
	}

	st.Docs.State, err = docsFreshness(root, m.Docs)
	return st, err
}

// overall folds the scopes into one state: anything behind makes the target
// behind. A scope whose tree is gone — the SDK on a clone that never checked
// it out — is not: its graph came with the repository and still answers.
func overall(scopes []workspace.ScopeState) workspace.Freshness {
	if len(scopes) == 0 {
		return workspace.Missing
	}
	state := workspace.Fresh
	for _, s := range scopes {
		switch s.State {
		case workspace.Stale, workspace.Missing:
			return workspace.Stale
		case workspace.Unknown:
			state = workspace.Unknown
		}
	}
	return state
}

func docsFreshness(root string, built *layout.Build) (workspace.Freshness, error) {
	if _, err := os.Stat(filepath.Join(root, docs.OutDir, docs.IndexFile)); errors.Is(err, fs.ErrNotExist) {
		return workspace.Missing, nil
	}
	if built == nil || built.Fingerprint == "" {
		return workspace.Unknown, nil
	}
	now, err := docs.Fingerprint(root)
	if err != nil {
		return "", err
	}
	if now != built.Fingerprint {
		return workspace.Stale, nil
	}
	return workspace.Fresh, nil
}

func printIndexStatus(out io.Writer, st IndexStatus) {
	target := func(name string, t TargetStatus, summary string) {
		fmt.Fprintf(out, "  %-5s %-8s %s", name, t.State, t.Dir)
		if t.Built != nil {
			fmt.Fprintf(out, "  %s", i18n.T("index.status.built", t.Built.At.Local().Format(time.DateTime), summary))
		}
		fmt.Fprintln(out)
	}

	codeSummary := ""
	if b := st.Code.Built; b != nil {
		codeSummary = i18n.T("index.status.code_counts", b.Counts.Scopes, b.Counts.Nodes, b.Counts.Edges)
	}
	target(layout.TargetCode, st.Code, codeSummary)
	for _, s := range st.Code.Scopes {
		fmt.Fprintf(out, "        %-12s %-11s %-8s %s\n", s.Name, s.Language, s.State, s.Root)
	}

	docsSummary := ""
	if b := st.Docs.Built; b != nil {
		docsSummary = i18n.T("index.status.docs_counts", b.Counts.Docs, b.Counts.Sections)
	}
	target(layout.TargetDocs, st.Docs, docsSummary)

	if st.Legacy {
		fmt.Fprintln(out, "\n  "+i18n.T("index.status.legacy"))
	}
	if !st.Current() {
		fmt.Fprintln(out, "\n  "+i18n.T("index.status.run"))
	}
}

// ensureDocsIndex rebuilds the document index unless the manifest shows it
// matches the documents. Only the index is written, never the markdown indexes
// the project versions: those are `gofi index docs`'s to change.
func ensureDocsIndex(root, language string) error {
	m, err := layout.LoadManifest(root)
	if err != nil {
		m = &layout.Manifest{}
	}
	state, err := docsFreshness(root, m.Docs)
	if err != nil || state == workspace.Fresh {
		return err
	}
	b := &docs.Builder{Root: root, WithCode: true, Language: language, IndexOnly: true}
	_, _, _, err = b.Build()
	if err != nil {
		return err
	}
	return recordDocsBuild(root, b)
}

// recordDocsBuild writes the document build to the manifest. The fingerprint
// is what the next read compares against.
func recordDocsBuild(root string, b *docs.Builder) error {
	idx, g, err := docs.Load(root)
	if err != nil {
		return err
	}
	sections := 0
	for _, d := range idx.Docs {
		sections += len(d.Sections)
	}
	return layout.Record(root, layout.TargetDocs, layout.Build{
		At: time.Now().UTC(), Tool: Version, Fingerprint: b.Fingerprint,
		Counts: layout.Counts{Docs: len(idx.Docs), Sections: sections, Nodes: len(g.Nodes), Edges: len(g.Edges)},
	})
}

// recordCodeBuild writes the code build to the manifest, with the scan options
// a later status check has to repeat.
func recordCodeBuild(root string, opt workspace.Options, ix *workspace.Index) error {
	if ix == nil || len(ix.Scopes) == 0 {
		return nil
	}
	c := layout.Counts{Scopes: len(ix.Scopes)}
	for _, s := range ix.Scopes {
		c.Nodes += s.Nodes
		c.Edges += s.Edges
		c.Files += s.Files
	}
	return layout.Record(root, layout.TargetCode, layout.Build{
		At:      time.Now().UTC(),
		Tool:    Version,
		Options: &layout.BuildOptions{WithTests: opt.WithTests, Exclude: opt.Exclude, MaxFileKB: opt.MaxFileKB},
		Counts:  c,
	})
}

// migrateLayout moves an index kept in the old layout to the current one and
// says what moved. The code graph is versioned, so the move shows up in git as
// files deleted and added — the note asks for it to be committed.
func migrateLayout(root, language string, out io.Writer) error {
	m, err := layout.Migrate(root, language)
	if err != nil || m.Empty() {
		return err
	}
	for _, mv := range m.Moved {
		fmt.Fprintln(out, i18n.T("index.layout.moved", mv[0], mv[1]))
	}
	for _, d := range m.Dropped {
		fmt.Fprintln(out, i18n.T("index.layout.dropped", d))
	}
	fmt.Fprintln(out, i18n.T("index.layout.commit"))
	return ensureGofiIgnored(root)
}
