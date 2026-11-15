package claude

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// line is the part of one stream-json output line the chat reads. Unknown
// types and fields are dropped on purpose: the engine emits bookkeeping (rate
// limits, hook lifecycle) the chat has nothing to say about, and new kinds
// appear between versions.
type line struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`

	SlashCommands []string `json:"slash_commands"`

	Event *struct {
		Type  string `json:"type"`
		Delta *struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	} `json:"event"`

	Message *struct {
		Content json.RawMessage `json:"content"`
		Usage   *usage          `json:"usage"`
	} `json:"message"`

	// ParentToolUseID names the tool call a subagent's message belongs to;
	// the conversation's own messages have none.
	ParentToolUseID *string `json:"parent_tool_use_id"`

	// Response is the engine's answer to a control request.
	Response *struct {
		Response *struct {
			Commands []struct {
				Name         string   `json:"name"`
				Description  string   `json:"description"`
				ArgumentHint string   `json:"argumentHint"`
				Aliases      []string `json:"aliases"`
				Builtin      bool     `json:"builtin"`
			} `json:"commands"`
		} `json:"response"`
	} `json:"response"`

	// CommandUUID and State are a command_lifecycle line: what became of a
	// message the engine was sent with an id.
	CommandUUID string `json:"command_uuid"`
	State       string `json:"state"`

	TerminalReason string              `json:"terminal_reason"`
	IsError        bool                `json:"is_error"`
	Result         string              `json:"result"`
	TotalCostUSD   float64             `json:"total_cost_usd"`
	DurationMS     float64             `json:"duration_ms"`
	Usage          *usage              `json:"usage"`
	ModelUsage     map[string]modelUse `json:"modelUsage"`
}

type usage struct {
	Input      int         `json:"input_tokens"`
	Output     int         `json:"output_tokens"`
	CacheRead  int         `json:"cache_read_input_tokens"`
	CacheWrite int         `json:"cache_creation_input_tokens"`
	Split      *cacheSplit `json:"cache_creation"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// translate maps one output line onto the events it carries. terminal reports
// that the line ended the turn.
func translate(raw []byte) (events []engine.Event, terminal bool) {
	var l line
	if json.Unmarshal(raw, &l) != nil {
		return nil, false // not JSON: engine chatter, not ours to render
	}
	switch l.Type {
	case "system":
		if l.Subtype == "init" {
			return []engine.Event{engine.Meta{SessionID: l.SessionID, Model: l.Model, SlashCommands: l.SlashCommands}}, false
		}

	case "control_response":
		if l.Response == nil || l.Response.Response == nil || len(l.Response.Response.Commands) == 0 {
			return nil, false
		}
		var meta engine.Meta
		for _, c := range l.Response.Response.Commands {
			meta.Commands = append(meta.Commands, engine.Command{
				Name: c.Name, Description: c.Description, Hint: c.ArgumentHint, Aliases: c.Aliases, Builtin: c.Builtin,
			})
		}
		return []engine.Event{meta}, false

	case "command_lifecycle":
		// "started" is the model taking a message that waited in the engine.
		if l.State == "started" && l.CommandUUID != "" {
			return []engine.Event{engine.Delivered{ID: l.CommandUUID}}, false
		}

	case "stream_event":
		if l.Event == nil || l.Event.Type != "content_block_delta" || l.Event.Delta == nil {
			return nil, false
		}
		sub := l.ParentToolUseID != nil
		switch l.Event.Delta.Type {
		case "text_delta":
			return []engine.Event{engine.Delta{Kind: engine.DeltaText, Text: l.Event.Delta.Text, Sub: sub}}, false
		case "thinking_delta":
			return []engine.Event{engine.Delta{Kind: engine.DeltaThinking, Text: l.Event.Delta.Thinking, Sub: sub}}, false
		}

	case "assistant":
		if l.Message == nil {
			return nil, false
		}
		msg := engine.Message{Parent: l.parent()}
		for _, b := range blocks(l.Message.Content) {
			switch b.Type {
			case "text":
				if b.Text != "" {
					msg.Blocks = append(msg.Blocks, engine.Block{Kind: engine.BlockText, Text: b.Text})
				}
			case "thinking":
				if b.Thinking != "" {
					msg.Blocks = append(msg.Blocks, engine.Block{Kind: engine.BlockThinking, Text: b.Thinking})
				}
			case "tool_use":
				msg.Blocks = append(msg.Blocks, engine.Block{Kind: engine.BlockToolUse, ID: b.ID, Name: b.Name, Input: b.Input})
			}
		}
		events = append(events, msg)
		if l.Message.Usage != nil {
			u := l.Message.Usage.event(false)
			u.Sub = l.ParentToolUseID != nil
			events = append(events, u)
		}
		return events, false

	case "user":
		if l.Message == nil {
			return nil, false
		}
		for _, b := range blocks(l.Message.Content) {
			if b.Type == "tool_result" {
				events = append(events, engine.ToolResult{ToolUseID: b.ToolUseID, IsError: b.IsError, Text: textOf(b.Content), Parent: l.parent()})
			}
		}
		return events, false

	case "result":
		if l.Usage != nil {
			events = append(events, l.Usage.event(true))
		}
		var split *cacheSplit
		if l.Usage != nil {
			split = l.Usage.Split
		}
		done := engine.Done{
			IsError:       l.IsError,
			Cancelled:     strings.HasPrefix(l.TerminalReason, "aborted"),
			CostUSD:       runCost(l.TotalCostUSD, l.ModelUsage, split),
			Duration:      time.Duration(l.DurationMS * float64(time.Millisecond)),
			ContextWindow: contextWindow(l.ModelUsage),
		}
		if done.Cancelled {
			done.IsError = false
		}
		if done.IsError {
			done.Err = firstNonEmpty(l.Result, l.Subtype, "the run failed")
		}
		return append(events, done), true
	}
	return nil, false
}

// contextWindow is the window of the model that read the most: the one
// holding the conversation, where a subagent ran on a smaller one.
func contextWindow(byModel map[string]modelUse) int {
	window, most := 0, -1
	for _, u := range byModel {
		if read := u.Input + u.CacheRead + u.CacheWrite; read > most {
			window, most = u.ContextWindow, read
		}
	}
	return window
}

// parent is the tool call that started the subagent a line is from.
func (l *line) parent() string {
	if l.ParentToolUseID == nil {
		return ""
	}
	return *l.ParentToolUseID
}

func (u *usage) event(total bool) engine.Usage {
	return engine.Usage{Total: total, Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite}
}

func blocks(raw json.RawMessage) []block {
	var out []block
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

// textOf flattens a tool result, which is either a string or a list of
// content blocks, to plain text.
func textOf(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []string
	for _, b := range blocks(raw) {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
