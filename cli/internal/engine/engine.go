// Package engine is the contract between the interactive mode and whatever
// runs the model behind it. The chat never talks to a vendor: it hands a turn
// to a Session and renders the normalised events that come back. Adding an
// engine means implementing Session — the chat stays untouched.
package engine

import (
	"context"
	"encoding/json"
	"time"
)

// Turn is one message from the user.
type Turn struct {
	Prompt string
}

// Session is one conversation with an engine. It is long-lived on purpose:
// engines pay a real startup cost (process spawn, project discovery, prompt
// cache warm-up) that must not be repeated per message.
type Session interface {
	// Send runs one turn and streams its events. The channel is closed after
	// exactly one terminal event, Done or Failure. Only one turn may be in
	// flight at a time.
	Send(ctx context.Context, turn Turn) (<-chan Event, error)
	// Cancel stops the turn in flight. The conversation survives it: the next
	// Send continues where this one was cut. Idempotent.
	Cancel()
	// Close releases the engine. The session is dead after it.
	Close() error
}

// Event is one thing that happened during a turn.
type Event interface{ isEvent() }

// Meta describes the engine once it is up: the conversation id to resume it
// by, the model answering, and the slash commands (skills) it knows.
type Meta struct {
	SessionID     string
	Model         string
	SlashCommands []string
}

// DeltaKind tells answer text from the model's thinking.
type DeltaKind int

const (
	DeltaText DeltaKind = iota
	DeltaThinking
)

// Delta is a piece of text as the model produces it, for streaming.
type Delta struct {
	Kind DeltaKind
	Text string
}

// BlockKind tells the blocks of a Message apart.
type BlockKind int

const (
	BlockText BlockKind = iota
	BlockThinking
	BlockToolUse
)

// Block is one part of a finished assistant message.
type Block struct {
	Kind  BlockKind
	Text  string          // BlockText, BlockThinking
	ID    string          // BlockToolUse
	Name  string          // BlockToolUse
	Input json.RawMessage // BlockToolUse
}

// Message is a finished assistant message. Its text blocks repeat what the
// deltas already streamed; its tool calls appear only here.
type Message struct {
	Blocks []Block
}

// ToolResult is what a tool call returned.
type ToolResult struct {
	ToolUseID string
	IsError   bool
	Text      string
}

// Usage counts tokens: per assistant message, or for the whole turn.
type Usage struct {
	Total      bool
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
}

// Done ends a turn that ran. IsError marks a run the engine itself reported as
// failed; Cancelled a turn the user stopped.
type Done struct {
	IsError   bool
	Cancelled bool
	Err       string
	CostUSD   float64
	Duration  time.Duration
}

// Failure ends a turn that could not run: the engine is missing, crashed or
// said nothing. Hint says what the user can do about it.
type Failure struct {
	Message string
	Hint    string
}

func (Meta) isEvent()       {}
func (Delta) isEvent()      {}
func (Message) isEvent()    {}
func (ToolResult) isEvent() {}
func (Usage) isEvent()      {}
func (Done) isEvent()       {}
func (Failure) isEvent()    {}

// Saved is a conversation the engine kept, which a Session can resume.
type Saved struct {
	ID      string
	Title   string
	Updated time.Time
}

// ModelSwitcher is a Session that can change model without losing the
// conversation. Optional: the chat offers /model only when the session is one.
// Warmer starts the engine ahead of the first turn, so its startup overlaps
// whatever runs before the message goes. Nothing is sent: it costs no token.
type Warmer interface {
	Warm()
}

type ModelSwitcher interface {
	SetModel(model string)
}
