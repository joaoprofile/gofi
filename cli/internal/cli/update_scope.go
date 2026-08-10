package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
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
}

type scopeLine struct {
	Path string
	Note string
}

func (s *updateScope) write(path, note string) {
	s.Writes = append(s.Writes, scopeLine{Path: path, Note: note})
}

// String renders the block. It is shown before the prompt and also under --yes,
// so a CI log records exactly what the run was allowed to touch.
func (s updateScope) String() string {
	var b strings.Builder
	b.WriteString("\nOn Yes:\n\n")

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
	}

	if len(s.LeavesAlone) > 0 {
		fmt.Fprintf(&b, "  LEAVES ALONE  %s\n", wrapList(s.LeavesAlone, 16, 62))
	}
	return b.String()
}

// confirm prints the scope and asks. autoConfirm answers yes without the
// prompt, but never without the block: skipping the question is not the same as
// hiding what happened.
func confirmUpdate(title string, s updateScope, autoConfirm bool) (bool, error) {
	fmt.Println(s)
	if autoConfirm {
		return true, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("this update requires --yes when stdin is not a TTY")
	}
	ok := false
	if err := huh.NewConfirm().
		Title(title).
		Description("Only what the block above lists as WRITES is touched.").
		Affirmative("Yes").
		Negative("No").
		Value(&ok).Run(); err != nil {
		return false, err
	}
	return ok, nil
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
