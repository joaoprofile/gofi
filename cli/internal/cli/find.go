package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/retrieval"
)

func newFindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <question>",
		Short: i18n.T("cmd.find.short"),
		Long: `Answer "where is it?" from the indexes, instead of grepping or opening files.

Documents and code are searched together: specs, PRDs and memory, the
reference libraries — portable knowledge, SDK docs and boilerplates, the
institutional base, the reference files of each skill — and the symbols of the
code graph. Each answer names the lines to read; 'gofi show' reads them.

Ranking is deterministic and offline: the words of the question are matched
against section headings, text, facets and paths, symbol names split into
words (BillOrder is "bill order"), Portuguese and English inflections folded
to one root, and a synonym bridge for the gap between the two languages: a
base lexicon of software terms shipped with gofi, plus the project's own in
.claude/lexicon/sinonimos.md.

--text looks for the exact text instead — an error message, a config key, a
SQL fragment — in every file of the project, like grep, and says which symbol
or section holds each line. --regex takes a regular expression, -i ignores
case. The code area is everything outside the document areas.

--in narrows the search to some areas. Nothing found usually means the wrong
words, not a missing document: try the technical term, and record the missing
pair in the lexicon.`,
		Example: `gofi find "ordem de delete chave estrangeira"
gofi find --in knowledge,sdk "paginação por cursor"
gofi find --in code "faturar pedido"
gofi find --json -n 3 "retry com backoff"
gofi find --text "ErrNotFound"
gofi find --regex -i "order_(item|line)s?" --in code`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, strings.Join(args, " "))
		},
	}
	cmd.Flags().IntP("limit", "n", 5, i18n.T("cmd.find.flag.limit"))
	cmd.Flags().StringSlice("in", nil, i18n.T("cmd.find.flag.in", strings.Join(retrieval.Areas, ", ")))
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	cmd.Flags().Bool("text", false, i18n.T("cmd.find.flag.text"))
	cmd.Flags().Bool("regex", false, i18n.T("cmd.find.flag.regex"))
	cmd.Flags().BoolP("ignore-case", "i", false, i18n.T("cmd.find.flag.ignore_case"))
	return cmd
}

// runSearch answers a question from documents and code together, through the
// retrieval core every surface shares.
func runSearch(cmd *cobra.Command, question string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	areas, _ := cmd.Flags().GetStringSlice("in")
	asJSON, _ := cmd.Flags().GetBool("json")
	for _, a := range areas {
		if !retrieval.ValidArea(a) {
			return fmt.Errorf("--in %q: use %s", a, strings.Join(retrieval.Areas, ", "))
		}
	}
	engine, err := openEngine()
	if err != nil {
		return err
	}
	text, _ := cmd.Flags().GetBool("text")
	regex, _ := cmd.Flags().GetBool("regex")
	if text || regex {
		// A line is smaller than a ranked answer, and a text search is read as
		// a list: the default shows more of them.
		if !cmd.Flags().Changed("limit") {
			limit = textLimit
		}
		ignoreCase, _ := cmd.Flags().GetBool("ignore-case")
		res, err := engine.Text(retrieval.TextQuery{Pattern: question, Regex: regex, IgnoreCase: ignoreCase, Areas: areas, Limit: limit})
		if err != nil {
			return err
		}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"schema": "gofi.find/v1", "mode": "text", "matches": res.Matches, "total": res.Total, "files": res.Files})
		}
		printTextMatches(cmd.OutOrStdout(), res)
		return nil
	}
	hits := engine.Find(retrieval.Query{Text: question, Areas: areas, Limit: limit})

	out := cmd.OutOrStdout()
	if asJSON {
		// Lists are never null in the contract: a consumer iterates, it does not
		// check for nil.
		missing := append([]string{}, engine.Missing...)
		return writeJSON(out, map[string]any{"schema": "gofi.find/v1", "mode": "rank", "hits": append([]retrieval.Hit{}, hits...), "missing": missing})
	}
	printHits(out, hits, engine.Missing)
	return nil
}

// printHits lists ranked answers: where to look and which lines to read.
func printHits(out io.Writer, hits []retrieval.Hit, missing []string) {
	if len(hits) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("find.none"))
		fmt.Fprintln(out, "  "+i18n.T("find.none_hint", docs.LexiconDir()))
	}
	for _, h := range hits {
		// Where it came from: the context when it has one, else the area —
		// "knowledge" tells the reader as much as a context name does.
		where := h.Area
		if h.Context != "" && h.Context != h.Area {
			where += " · " + h.Context
		}
		fmt.Fprintf(out, "  [%4.1f] %s  (%s)\n", h.Score, h.Path, where)
		lines := fmt.Sprintf("L%d-%d", h.Start, h.End)
		if h.End <= h.Start {
			lines = fmt.Sprintf("L%d", h.Start)
		}
		switch h.Kind {
		case retrieval.KindSymbol:
			fmt.Fprintf(out, "         ◆ %s  %s  %s\n", h.Title, lines, h.Detail)
		default:
			fmt.Fprintf(out, "         §%s  %s\n", h.Title, lines)
		}
		if h.Overrides != "" {
			fmt.Fprintln(out, "         "+i18n.T("find.overrides", h.Overrides))
		}
		if h.OverriddenBy != "" {
			fmt.Fprintln(out, "         "+i18n.T("find.overridden_by", h.OverriddenBy))
		}
	}
	for _, m := range missing {
		fmt.Fprintln(out, "  "+i18n.T("find.missing", m))
	}
}

// textLimit is how many matching lines a text search shows by default.
const textLimit = 30

// printTextMatches lists the matches grouped by the unit that holds them, so a
// run of hits inside one function reads as one place to open, not as ten.
func printTextMatches(out io.Writer, res *retrieval.TextResult) {
	if res.Total == 0 {
		fmt.Fprintln(out, "  "+i18n.T("find.text.none"))
		return
	}
	width := 0
	for _, m := range res.Matches {
		width = max(width, len(strconv.Itoa(m.Line)))
	}
	group := ""
	for _, m := range res.Matches {
		head := m.Path
		if in := m.In; in != nil {
			mark := "§"
			if in.Kind == retrieval.KindSymbol {
				mark = "◆ "
			}
			head += fmt.Sprintf("  %s%s  L%d-%d", mark, in.Title, in.Start, max(in.End, in.Start))
		}
		if head != group {
			fmt.Fprintf(out, "  %s\n", head)
			group = head
		}
		fmt.Fprintf(out, "    %*d  %s\n", width, m.Line, m.Text)
	}
	if shown := len(res.Matches); shown < res.Total {
		fmt.Fprintln(out, "\n  "+i18n.T("find.text.more", shown, res.Total, res.Files))
	} else {
		fmt.Fprintln(out, "\n  "+i18n.T("find.text.count", res.Total, res.Files))
	}
}
