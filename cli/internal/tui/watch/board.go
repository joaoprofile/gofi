package watch

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// Board is what is running in a project right now: its live runs, each with
// the agents at work in it. It is the screen's first glance, made to be
// pinned over something else — the chat draws it above the conversation while
// a run goes. Like the screen, it only reads the disk: it shows the same
// whoever started the run, and asks no model anything.
type Board struct {
	tabs    []tab
	root    string
	at      time.Time
	Running int     // runs at work
	Waiting int     // runs stopped at a gate, waiting on the person
	Working int     // agents at work, all runs together
	Cost    float64 // what the live runs have spent
}

// Live reads the runs that are alive under root.
func Live(root string) Board {
	b := Board{root: root, at: time.Now()}
	infos, _ := loop.Live(root)
	var tabs []tab
	for _, info := range infos {
		run := info.State.Run
		t := tab{info: info}
		t.agents, _ = loop.Agents(root, run)
		if q, ok := loop.OpenGate(root, run); ok {
			t.gate = q
		}
		t.shape(readFlow(loop.FlowFile(root, run)))
		tabs = append(tabs, t)
		if info.Liveness == loop.Blocked {
			b.Waiting++
		} else {
			b.Running++
		}
		b.Cost += info.State.CostUSD
		for _, a := range t.agents {
			if a.Status == loop.AgentWorking {
				b.Working++
			}
		}
	}
	// What waits on the person comes first, as on the screen.
	for _, g := range groupTabs(tabs) {
		for _, i := range g.tabs {
			b.tabs = append(b.tabs, tabs[i])
		}
	}
	return b
}

// Active reports whether anything is running or waiting.
func (b Board) Active() bool { return len(b.tabs) > 0 }

// Lines draws the board w columns wide in at most limit lines, the last one
// a rule that sets it apart from what is under it. Nothing when nothing runs.
func (b Board) Lines(w, limit int) []string {
	if !b.Active() || limit < 3 {
		return nil
	}
	k := newLook()
	w = max(w, 20)
	head := k.accent.Render("●") + " " + k.bold.Render(i18n.T("board.title")) + k.dim.Render(" · ") + strings.Join(b.Facts(), k.dim.Render(" · "))
	out := []string{spread(head, k.dim.Render(i18n.T("board.hint")), w)}
	out = append(out, b.Rows(w, limit-2)...)
	return append(out, k.border.Render(strings.Repeat("─", w)))
}

// Facts are the board in numbers: how many runs, how many agents at work,
// how many wait on the person, what they have spent. None when nothing runs.
func (b Board) Facts() []string {
	if !b.Active() {
		return nil
	}
	k := newLook()
	facts := []string{i18n.T("board.runs", len(b.tabs))}
	if b.Working > 0 {
		facts = append(facts, i18n.T("board.working", b.Working))
	}
	if b.Waiting > 0 {
		facts = append(facts, k.warn.Render(i18n.T("board.waiting", b.Waiting)))
	}
	return append(facts, money(b.Cost))
}

// Rows are the runs alone, for a panel with a head of its own: each run on a
// line — its progress through the flow, what it has spent — and under it the
// agents at work, each with the last thing it did. At most limit lines.
func (b Board) Rows(w, limit int) []string {
	if !b.Active() || limit < 1 {
		return nil
	}
	k := newLook()
	w = max(w, 20)
	var body []string
	for _, t := range b.tabs {
		s := t.info.State
		name := s.Context
		if name == "" {
			name = s.Change
		}
		left := " " + k.mark(runState(t.info)) + " " + k.bold.Render(name)
		if strip := k.strip(t); strip != "" {
			left += "  " + strip
		} else {
			left += "  " + k.dim.Render(say(t))
		}
		body = append(body, spread(left, k.dim.Render(money(s.CostUSD)), w))
		for _, a := range t.agents {
			if a.Status != loop.AgentWorking {
				continue
			}
			line := "     " + k.accent.Render("●") + fmt.Sprintf(" #%d %s", a.Iter, a.Role) +
				k.dim.Render(fmt.Sprintf(" · %s · %s", a.Tier, since(b.at.Sub(a.Started))))
			if doing := activity(loop.Transcript(b.root, s.Run, a.ID), s.Run); doing != "" {
				line += "  " + k.dim.Render(doing)
			}
			body = append(body, ansi.Truncate(line, w, "…"))
		}
		if t.gate != nil {
			body = append(body, ansi.Truncate("     "+k.warn.Render("! "+i18n.T("board.gate", len(t.gate.Questions))), w, "…"))
		}
	}
	if len(body) > limit {
		hidden := len(body) - limit + 1
		body = append(body[:limit-1], k.dim.Render("     "+i18n.T("board.more", hidden)))
	}
	return body
}

// since is how long an agent has been at it, the way a glance wants it.
func since(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 0:
		return "0s"
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh%02dm", s/3600, s%3600/60)
}

// activityTail is how much of a transcript's end is read for its last step.
const activityTail = 16 << 10

// activity is the last thing an agent did, in a few words: the tool it
// called and on what, or the start of what it wrote. Only the end of the
// transcript is read — it is asked for every second.
func activity(path, run string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > activityTail {
		_, _ = f.Seek(-activityTail, io.SeekEnd)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var l struct{ Kind, Text string }
		if json.Unmarshal([]byte(lines[i]), &l) != nil {
			continue
		}
		switch l.Kind {
		case "tool":
			return toolLine(l.Text, run)
		case "text":
			if t := strings.TrimSpace(l.Text); t != "" {
				return strings.Join(strings.Fields(strings.SplitN(t, "\n", 2)[0]), " ")
			}
		}
	}
	return ""
}

// toolLine is a recorded tool call — its name, then its input as JSON — as
// the name and the one thing worth reading about it. A path loses the run's
// worktree, so what shows is the file.
func toolLine(text, run string) string {
	name, input, _ := strings.Cut(text, " ")
	var args map[string]any
	if json.Unmarshal([]byte(input), &args) != nil {
		return name
	}
	for _, key := range []string{"file_path", "notebook_path", "command", "pattern", "description", "query", "url", "skill", "prompt"} {
		v, ok := args[key].(string)
		if !ok || strings.TrimSpace(v) == "" {
			continue
		}
		if _, after, found := strings.Cut(v, "/"+run+"/"); found && strings.HasSuffix(key, "path") {
			v = after
		}
		return name + " " + strings.Join(strings.Fields(v), " ")
	}
	return name
}
