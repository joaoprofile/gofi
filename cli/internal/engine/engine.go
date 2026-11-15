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
	// Images go with the prompt, ahead of its text.
	Images []Image
}

// Image is a picture the user attached to a turn.
type Image struct {
	MediaType string // image/png, image/jpeg, image/gif or image/webp
	Data      []byte
}

// Command is a slash command the engine runs, as it describes itself.
type Command struct {
	Name        string
	Description string
	Hint        string // what follows the name, e.g. "<model>"
	Aliases     []string
	Builtin     bool // the engine's own, rather than a skill of the project or the person
}

// Session is one conversation with an engine. It is long-lived on purpose:
// engines pay a real startup cost (process spawn, project discovery, prompt
// cache warm-up) that must not be repeated per message.
type Session interface {
	// Send runs one turn and streams its events. The channel is closed after
	// a terminal event, Done or Failure — the only one, unless a message sent
	// during the turn (Steerer) was run by the engine as a turn of its own:
	// then each has its Done, and the channel closes after the last. Only one
	// turn may be in flight at a time.
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
	// Commands are the slash commands with what each one does, when the
	// engine says so; SlashCommands carries the names alone.
	Commands []Command
}

// DeltaKind tells answer text from the model's thinking.
type DeltaKind int

const (
	DeltaText DeltaKind = iota
	DeltaThinking
)

// Delta is a piece of text as the model produces it, for streaming. Sub
// marks a subagent's: its answer goes to the agent that asked, not to the
// person.
type Delta struct {
	Kind DeltaKind
	Text string
	Sub  bool
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
// deltas already streamed; its tool calls appear only here. Parent is the
// tool call that started the subagent a message is from; the conversation's
// own have none.
type Message struct {
	Blocks []Block
	Parent string
}

// ToolResult is what a tool call returned. Parent is as in Message.
type ToolResult struct {
	ToolUseID string
	IsError   bool
	Text      string
	Parent    string
}

// Delivered says the model was handed a message sent during the turn
// (Steerer): from here on it has read it.
type Delivered struct {
	ID string
}

// Usage counts tokens: per assistant message, or for the whole turn.
type Usage struct {
	Total bool
	// Sub marks a subagent's message: what it read is its own context, not
	// the conversation's.
	Sub        bool
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
	// CostUSD is what the engine's process has spent since it started: a
	// session of several turns reports the running total on each.
	CostUSD  float64
	Duration time.Duration
	// ContextWindow is how many tokens the answering model holds, when the
	// engine says so.
	ContextWindow int
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
func (Delivered) isEvent()  {}
func (Usage) isEvent()      {}
func (Done) isEvent()       {}
func (Failure) isEvent()    {}

// Saved is a conversation the engine kept, which a Session can resume.
type Saved struct {
	ID      string
	Title   string
	Updated time.Time
}

// Warmer starts the engine ahead of the first turn, so its startup overlaps
// whatever runs before the message goes. Nothing is sent: it costs no token.
type Warmer interface {
	Warm()
}

// ModelSwitcher is a Session that can change model without losing the
// conversation. Optional: the chat offers /model only when the session is one.
type ModelSwitcher interface {
	SetModel(model string)
}

// Steerer is a Session that takes a message while a turn is in flight. The
// engine keeps it and hands it to the model at its next step — between two
// tool calls —, so the turn goes on knowing it; with no step left, the engine
// runs it as a turn of its own right after. Either way it costs the message
// and nothing else: no request is made that would not be made anyway.
type Steerer interface {
	// Steer sends the message and returns the id its Delivered event will
	// carry. False when no turn is in flight: send it as a turn.
	Steer(turn Turn) (id string, ok bool)
}

// Describer is a Session whose engine says what it offers before the first
// turn: its slash commands, each with what it does. It starts the engine if
// it must and waits for the answer until ctx ends. No model is called: it
// costs no token.
type Describer interface {
	Describe(ctx context.Context) (Meta, bool)
}
