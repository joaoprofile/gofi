package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

func newVerdictCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verdict",
		Short: i18n.T("cmd.verdict.short"),
		Long: `The audit's result as data: what the QA found, each finding with a severity,
the owner of the fix and a fingerprint — the file the loop routes on and the
person reads (decision 0002, D5).`,
		Example: `gofi verdict write pedidos < verdict.json`,
	}
	write := &cobra.Command{
		Use:   "write <context>",
		Short: i18n.T("cmd.verdict.write.short"),
		Long: `Write a context's verdict, read from stdin as JSON (` + verdict.Schema + `), into
.gofi/verdict/<context>.json — and status and ressalvas into the context's
memory, so both say the same thing.

Checked before it is written: result is aprovado or reprovado; a blocker
rejects the delivery, and a rejection needs a blocker, major or minor; each
finding has a severity (blocker, major, minor, suggestion), an owner (eng,
spec, pd), a rule and a message. gofi fills in ids, the schema and every
fingerprint — rule, symbol or file, and category, never the line — so a
finding is recognised when it comes back.`,
		Example: `gofi verdict write pedidos <<'EOF'
{"result": "reprovado", "findings": [
  {"severity": "major", "rule": "pedidos/REQ-04.1", "symbol": "pedidos/service.Cancel",
   "owner": "eng", "message": "cancelamento após envio não é recusado"}
]}
EOF`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			if !contextName.MatchString(args[0]) {
				return fmt.Errorf("%q is not a context name (lowercase letters, digits, - and _)", args[0])
			}
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return err
			}
			v, err := verdict.Parse(data, args[0])
			if err != nil {
				return &ExitCodeError{Code: 1, Err: err}
			}
			if run, _ := cmd.Flags().GetString("run"); run != "" {
				v.Run = run
			}
			if iter, _ := cmd.Flags().GetInt("iter"); iter > 0 {
				v.Iter = iter
			}
			path, err := verdict.Write(root, v)
			if err != nil {
				return err
			}
			if err := recordVerdict(root, v); err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("verdict.written", filepath.ToSlash(rel), v.Result, v.Caveats(), len(v.Findings)))
			return nil
		},
	}
	write.Flags().String("run", "", i18n.T("cmd.verdict.flag.run"))
	write.Flags().Int("iter", 0, i18n.T("cmd.verdict.flag.iter"))
	cmd.AddCommand(write)
	return cmd
}

// recordVerdict carries the verdict into the context's memory: status and
// ressalvas, the fields every conductor and gofi-status read.
func recordVerdict(root string, v *verdict.Verdict) error {
	path := layout.Contexts().Abs(root, v.Context+".md")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	out, err := docs.SetFields(string(raw), [][2]string{
		{"status", v.Result},
		{"ressalvas", fmt.Sprint(v.Caveats())},
		{"atualizado", time.Now().Format("2006-01-02")},
	})
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
