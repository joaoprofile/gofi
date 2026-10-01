package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/retrieval"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <ref>",
		Short: i18n.T("cmd.show.short"),
		Long: `Describe what a reference names, from the indexes — instead of opening files.

The kind comes from the reference:

  a path ending in .md          the document: outline with line ranges, links in and out
  path.md#heading  or  #L42     one section: only its lines, numbered as in the file
  a symbol (Bill, service.Bill) what it is, where, what it calls and what calls it
  ctx:<name>                    everything filed under a context — specs, PRD,
                                memory, tables, code and the gaps between them
  table:<name>                  the documents that own a table and the code that touches it

A name that could be two things — a context and a table, two symbols — is
listed with the prefixed references to pick from (sym:, doc:, ctx:, table:);
nothing is guessed. Use 'gofi find' first when you have words, not a name.`,
		Example: `gofi show specs/order/sdd-order.md
gofi show "specs/order/sdd-order.md#RN-01"
gofi show service.Bill
gofi show ctx:order
gofi show table:order_order`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, err := openEngine()
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetInt("limit")
			view, err := engine.Show(args[0], limit)
			return printView(cmd, view, err)
		},
	}
	cmd.Flags().IntP("limit", "n", 12, i18n.T("cmd.show.flag.limit"))
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	return cmd
}

func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path <from> <to>",
		Short: i18n.T("cmd.path.short"),
		Long: `Show how one symbol reaches another in the code graph: every hop, with where
in the code it happens. The answer to "how does A end up calling B" without
reading the files in between.

Only as complete as the graph: in fast mode an ambiguous call has no edge, so a
missing path is not proof that none exists — build with 'gofi index code --deep'
before concluding that.`,
		Example: `gofi path handler.Login store.User
gofi path OrderHandler.Bill Repository.Save`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, err := openEngine()
			if err != nil {
				return err
			}
			view, err := engine.Path(args[0], args[1])
			return printView(cmd, view, err)
		},
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	return cmd
}

// openEngine opens the retrieval core on the project the index commands
// build, reading the backend language from .gofi.yaml when it can. A broken
// config does not stop a read: the indexes on disk are still there.
//
// The document index is brought up to date first. It is not versioned — it is
// rebuilt in about a second from files that are — so a fresh clone has none,
// and any edit to a document leaves it behind. Rebuilding here is what lets
// both be true: nobody commits it, and no query answers from a stale one.
func openEngine() (*retrieval.Engine, error) {
	root, err := graphProjectRoot()
	if err != nil {
		return nil, err
	}
	language := ""
	if cfg, _, err := loadProjectConfig(); err == nil {
		language = backendLanguage(cfg)
	}
	if err := ensureDocsIndex(root, language); err != nil {
		return nil, err
	}
	return retrieval.Open(root, language)
}

// printView writes a view as text or, with --json, in the versioned contract
// every tool reads. Not finding something is an answer, not a failure: it is
// printed and the command exits 0.
func printView(cmd *cobra.Command, view *retrieval.View, err error) error {
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON && err == nil {
		return writeJSON(out, map[string]any{"schema": "gofi.show/v1", "kind": view.Kind, "ref": view.Ref, "data": view.Data})
	}
	return renderView(out, view, err)
}

// renderView writes a view as text, with a reference that names nothing
// reported as an answer rather than a failure.
func renderView(out io.Writer, view *retrieval.View, err error) error {
	if errors.Is(err, retrieval.ErrNotFound) {
		fmt.Fprintln(out, "  "+err.Error())
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprint(out, view.Text)
	return nil
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
