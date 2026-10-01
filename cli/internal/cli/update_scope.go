package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/tui/flow"
)

// updateScope is the answer every `gofi update <target>` owes the user before
// it writes anything: what it writes, what it keeps because the project made it
// its own, and — the one that matters most here — what it leaves alone.
//
// After `gofi init` the project belongs to the team, so "what else will this
// move?" is the question the confirmation exists to answer. A target that
// cannot fill LeavesAlone is a target whose scope was never decided.
type updateScope struct {
	// Writes lists the paths about to be written, project-relative, each with a
	// short note ("9 file(s): 2 new, 3 changed").
	Writes []scopeLine
	// Keeps lists the files the update will not overwrite because they carry
	// local edits. Under Force these are what it is about to replace instead.
	Keeps []string
	// LeavesAlone names what stays exactly as it is.
	LeavesAlone []string
	// Force flips Keeps from a promise into a warning.
	Force bool
	// Replaces marks a target that overwrites wholesale (the institutional
	// mirror), where "keeps" would be a lie.
	Replaces bool
	// Hint says what to do about the kept files, shown under KEEPS. Set by the
	// targets of the gofi zone, where an edited file stops receiving fixes.
	Hint string
}

// keepsHint is the Hint of every gofi-zone target: an edit to a file gofi owns
// freezes it, and the team's rule belongs in knowledge/, where it wins.
const keepsHint = "an edited file stops getting fixes — move the difference to " +
	"knowledge/ with overrides: in its frontmatter, then --force to take upstream back"

type scopeLine struct {
	Path string
	Note string
}

func (s *updateScope) write(path, note string) {
	s.Writes = append(s.Writes, scopeLine{Path: path, Note: note})
}

// atRisk reports whether this run can cost the user something upstream cannot
// give back. That — not the mere fact of writing — is what earns a prompt.
//
// Two cases qualify. A mirror replaces wholesale, so whatever is there is gone.
// And --force is the flag whose entire purpose is to overwrite the files you
// edited: with none of those on disk it destroys nothing, so it does not ask
// either. Everything else the user already decided by naming the target;
// asking again is ceremony, and a prompt that always appears is a prompt nobody
// reads.
func (s updateScope) atRisk() bool {
	return s.Replaces || (s.Force && len(s.Keeps) > 0)
}

// String renders the block. It is shown whether or not a prompt follows, and
// also under --yes, so a CI log records exactly what the run was allowed to
// touch.
func (s updateScope) String() string {
	var b strings.Builder
	header := "This run:"
	if s.atRisk() {
		header = "On Yes:"
	}
	b.WriteString("\n" + header + "\n\n")

	width := 0
	for _, w := range s.Writes {
		if l := len(w.Path); l > width {
			width = l
		}
	}
	for _, w := range s.Writes {
		line := fmt.Sprintf("  WRITES        %-*s", width, w.Path)
		if w.Note != "" {
			line += "  " + w.Note
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	if len(s.Writes) == 0 {
		b.WriteString("  WRITES        nothing — everything is already current\n")
	}

	switch {
	case s.Replaces:
		b.WriteString("  KEEPS         nothing — this target is an authoritative mirror\n")
	case len(s.Keeps) > 0 && s.Force:
		fmt.Fprintf(&b, "  OVERWRITES    %d file(s) you edited — copy kept in .gofi/backup/\n", len(s.Keeps))
	case len(s.Keeps) > 0:
		fmt.Fprintf(&b, "  KEEPS         %d file(s) you edited\n", len(s.Keeps))
		if s.Hint != "" {
			fmt.Fprintf(&b, "                %s\n", wrapWords(s.Hint, 16, 62))
		}
	}

	if len(s.LeavesAlone) > 0 {
		fmt.Fprintf(&b, "  LEAVES ALONE  %s\n", wrapList(s.LeavesAlone, 16, 62))
	}
	return b.String()
}

// confirmUpdate prints the scope and asks only when the run can destroy
// something. Reports whether to proceed.
//
// The block prints every time — including under --yes and when no prompt
// follows. Not asking is not the same as not saying: "what else will this
// move?" deserves an answer whether or not there is a question after it.
func confirmUpdate(title string, s updateScope, autoConfirm bool) (bool, error) {
	fmt.Println(s)
	if len(s.Writes) == 0 {
		// Nothing to do, and the block already said so. Asking whether to do
		// nothing is the most annoying prompt there is.
		return false, nil
	}
	if autoConfirm || !s.atRisk() {
		return true, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("this update requires --yes when stdin is not a TTY")
	}
	ok, err := flow.YesNo(title, i18n.T("update.confirm_help"), "", "", false)
	if errors.Is(err, flow.ErrCancelled) {
		return false, nil
	}
	return ok, err
}

// wrapList joins items with commas, folding onto continuation lines indented by
// indent so a long LEAVES ALONE list stays readable in a narrow terminal.
func wrapList(items []string, indent, width int) string {
	var b strings.Builder
	line := 0
	for i, it := range items {
		piece := it
		if i < len(items)-1 {
			piece += ","
		}
		if line > 0 && line+len(piece)+1 > width {
			b.WriteString("\n" + strings.Repeat(" ", indent))
			line = 0
		} else if line > 0 {
			b.WriteString(" ")
			line++
		}
		b.WriteString(piece)
		line += len(piece)
	}
	return b.String()
}

// planNote summarises a skills plan for the WRITES line: how many files change
// and in what way, since the list itself was printed above.
func planNote(newFiles, changed int) string {
	switch {
	case newFiles == 0 && changed == 0:
		return "nothing to change"
	case newFiles == 0:
		return fmt.Sprintf("%d changed", changed)
	case changed == 0:
		return fmt.Sprintf("%d new", newFiles)
	}
	return fmt.Sprintf("%d new, %d changed", newFiles, changed)
}

// wrapWords folds text onto continuation lines indented by indent, breaking
// between words.
func wrapWords(text string, indent, width int) string {
	var b strings.Builder
	line := 0
	for _, w := range strings.Fields(text) {
		if line > 0 && line+len(w)+1 > width {
			b.WriteString("\n" + strings.Repeat(" ", indent))
			line = 0
		} else if line > 0 {
			b.WriteString(" ")
			line++
		}
		b.WriteString(w)
		line += len(w)
	}
	return b.String()
}
