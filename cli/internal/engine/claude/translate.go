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

	IsError      bool                `json:"is_error"`
	Result       string              `json:"result"`
	TotalCostUSD float64             `json:"total_cost_usd"`
	DurationMS   float64             `json:"duration_ms"`
	Usage        *usage              `json:"usage"`
	ModelUsage   map[string]modelUse `json:"modelUsage"`
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

	case "stream_event":
		if l.Event == nil || l.Event.Type != "content_block_delta" || l.Event.Delta == nil {
			return nil, false
		}
		switch l.Event.Delta.Type {
		case "text_delta":
			return []engine.Event{engine.Delta{Kind: engine.DeltaText, Text: l.Event.Delta.Text}}, false
		case "thinking_delta":
			return []engine.Event{engine.Delta{Kind: engine.DeltaThinking, Text: l.Event.Delta.Thinking}}, false
		}

	case "assistant":
		if l.Message == nil {
			return nil, false
		}
		var msg engine.Message
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
			events = append(events, l.Message.Usage.event(false))
		}
		return events, false

	case "user":
		if l.Message == nil {
			return nil, false
		}
		for _, b := range blocks(l.Message.Content) {
			if b.Type == "tool_result" {
				events = append(events, engine.ToolResult{ToolUseID: b.ToolUseID, IsError: b.IsError, Text: textOf(b.Content)})
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
			IsError:  l.IsError,
			CostUSD:  runCost(l.TotalCostUSD, l.ModelUsage, split),
			Duration: time.Duration(l.DurationMS * float64(time.Millisecond)),
		}
		if l.IsError {
			done.Err = firstNonEmpty(l.Result, l.Subtype, "the run failed")
		}
		return append(events, done), true
	}
	return nil, false
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
