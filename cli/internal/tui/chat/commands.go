package chat

import (
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/gofi-labs/gofi/cli/internal/engine"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
)

// replaceSession closes the conversation and opens another — a new one, or
// the saved one with resumeID — keeping the model picked with /model.
func (m *model) replaceSession(resumeID string) {
	_ = m.session.Close()
	m.session = m.opts.NewSession(resumeID)
	if sw, ok := m.session.(engine.ModelSwitcher); ok && m.chosenModel != "" {
		sw.SetModel(m.chosenModel)
	}
	m.tools = map[string]string{}
	m.always = map[string]bool{}
}

// switchModel is /model: without an argument it lists the choices; with a
// number or a name it switches, and the conversation carries on.
func (m *model) switchModel(arg string) string {
	k := m.look
	sw, ok := m.session.(engine.ModelSwitcher)
	if !ok {
		return k.nested([]string{i18n.T("chat.model.unsupported")}, k.warn)
	}
	if arg == "" {
		lines := []string{i18n.T("chat.model.current", m.modelN), ""}
		for i, id := range m.opts.Models {
			lines = append(lines, k.accent.Render(fmt.Sprintf("%2d.", i+1))+" "+id)
		}
		lines = append(lines, "", i18n.T("chat.model.pick"))
		return "\n" + k.nested(lines, k.text)
	}
	id := arg
	if n, err := strconv.Atoi(arg); err == nil {
		if n < 1 || n > len(m.opts.Models) {
			return k.nested([]string{i18n.T("chat.model.unknown", arg)}, k.bad)
		}
		id = m.opts.Models[n-1]
	}
	sw.SetModel(id)
	m.chosenModel, m.modelN = id, id
	return k.nested([]string{i18n.T("chat.model.set", id)}, k.good)
}

// resume is /resume: without an argument it lists the saved conversations of
// this folder; with a number from that list, or an id, it continues one.
func (m *model) resume(arg string) string {
	k := m.look
	if m.opts.Sessions == nil {
		return k.nested([]string{i18n.T("chat.resume.none")}, k.dim)
	}
	if arg == "" {
		m.listed = m.opts.Sessions()
		if len(m.listed) == 0 {
			return k.nested([]string{i18n.T("chat.resume.none")}, k.dim)
		}
		var lines []string
		for i, s := range m.listed {
			when := k.dim.Render(ago(time.Since(s.Updated)))
			title := ansi.Truncate(s.Title, max(m.width-18, 20), "…")
			lines = append(lines, k.accent.Render(fmt.Sprintf("%2d.", i+1))+" "+title+"  "+when)
		}
		lines = append(lines, "", i18n.T("chat.resume.pick"))
		return "\n" + k.nested(lines, k.text)
	}

	target := engine.Saved{ID: arg, Title: arg}
	if n, err := strconv.Atoi(arg); err == nil {
		if n < 1 || n > len(m.listed) {
			return k.nested([]string{i18n.T("chat.resume.unknown", arg)}, k.bad)
		}
		target = m.listed[n-1]
	}
	m.replaceSession(target.ID)
	return k.nested([]string{i18n.T("chat.resume.done", target.Title)}, k.good)
}

// ago says how long ago, the way a list of conversations wants it.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return i18n.T("chat.ago.now")
	case d < time.Hour:
		return i18n.T("chat.ago.minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.T("chat.ago.hours", int(d.Hours()))
	}
	return i18n.T("chat.ago.days", int(d.Hours()/24))
}
