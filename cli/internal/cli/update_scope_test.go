package cli

import (
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/audit"
)

// The block is the promise the whole update family rests on: before writing
// anything, say what moves and — the question the user actually has — what does
// not. A target whose block cannot answer the third line never decided its
// scope.
func TestUpdateScope_AnswersAllThreeQuestions(t *testing.T) {
	s := updateScope{
		Keeps:       []string{".claude/skills/gofi-eng/SKILL.md"},
		LeavesAlone: []string{".gofi.yaml", "memory/"},
	}
	s.write(".claude/skills/", "9 file(s): 2 new, 3 changed")

	out := s.String()
	for _, want := range []string{
		"On Yes:",
		"WRITES        .claude/skills/  9 file(s): 2 new, 3 changed",
		"KEEPS         1 file(s) you edited",
		"LEAVES ALONE  .gofi.yaml, memory/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("block is missing %q:\n%s", want, out)
		}
	}
}

// Under --force the same files flip from a promise to a warning, and the block
// has to read as one — this is the run that can lose work.
func TestUpdateScope_ForceWarnsInsteadOfPromising(t *testing.T) {
	s := updateScope{Keeps: []string{"a", "b"}, Force: true, LeavesAlone: []string{"memory/"}}
	s.write(".claude/skills/", "")

	out := s.String()
	if !strings.Contains(out, "OVERWRITES    2 file(s) you edited — copy kept in .gofi/backup/") {
		t.Errorf("--force must warn about what it replaces:\n%s", out)
	}
	if strings.Contains(out, "KEEPS") {
		t.Errorf("--force keeps nothing; the line must not say otherwise:\n%s", out)
	}
}

// The institutional mirror is the one target that keeps nothing, and saying
// "KEEPS 0 file(s)" would read as a guarantee it does not offer.
func TestUpdateScope_MirrorSaysItKeepsNothing(t *testing.T) {
	s := updateScope{Replaces: true, LeavesAlone: []string{"specs/"}}
	s.write(".claude/institutional/svc/", "full wipe + recopy")

	if !strings.Contains(s.String(), "KEEPS         nothing — this target is an authoritative mirror") {
		t.Errorf("a mirror must say it keeps nothing:\n%s", s)
	}
}

// A run with nothing to do still prints a block, so "I said yes and nothing
// happened" is never a surprise.
func TestUpdateScope_EmptyIsStillExplicit(t *testing.T) {
	s := updateScope{LeavesAlone: []string{"everything"}}
	if !strings.Contains(s.String(), "WRITES        nothing — everything is already current") {
		t.Errorf("an empty scope must say so:\n%s", s)
	}
}

func TestWrapList_FoldsLongLists(t *testing.T) {
	got := wrapList([]string{"aaaa", "bbbb", "cccc", "dddd"}, 4, 12)
	if !strings.Contains(got, "\n    ") {
		t.Errorf("a list past the width must fold with the given indent, got %q", got)
	}
	if strings.Contains(got, "dddd,") {
		t.Errorf("the last item must not carry a comma, got %q", got)
	}
}

// Splitting the update into targets cost the project the report that used to
// run at the end of every update. The footer is what replaces it, so it has to
// carry the number that decides whether to bother — and stay silent when there
// is nothing to say.
func TestDriftNote(t *testing.T) {
	if got := driftNote(nil); got != "" {
		t.Errorf("a clean project must not pay for the line, got %q", got)
	}

	info := audit.Finding{Severity: audit.SeverityInfo}
	warn := audit.Finding{Severity: audit.SeverityWarn}

	got := driftNote([]audit.Finding{info, info})
	if !strings.Contains(got, "2 structure finding(s)") || strings.Contains(got, "needing attention") {
		t.Errorf("info-only drift should report a plain count, got %q", got)
	}
	if !strings.Contains(got, "gofi update audit") {
		t.Errorf("the note must name the command that prints the rest, got %q", got)
	}

	got = driftNote([]audit.Finding{info, warn, warn})
	if !strings.Contains(got, "3 structure finding(s), 2 needing attention") {
		t.Errorf("warnings must be counted apart from the total, got %q", got)
	}
}
