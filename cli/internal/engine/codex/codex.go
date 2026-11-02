// Package codex runs OpenAI's Codex CLI as an engine.Session: one
// `codex exec --json` per turn, its JSON Lines events read into the engine's.
// The conversation goes on with `codex exec resume <thread>`.
package codex

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/engine/oneshot"
)

// DefaultExecutable is the CLI looked up on PATH.
const DefaultExecutable = "codex"

// Options configures a session.
type Options struct {
	Executable string // DefaultExecutable when empty
	Dir        string // the project root the engine runs in
	Model      string // passed as --model when set
	// Sandbox is the policy for the commands the model runs; workspace-write
	// when empty — edits in the project, nothing outside it.
	Sandbox string
}

// New opens a session; nothing runs until the first turn.
func New(o Options) *oneshot.Session {
	if o.Executable == "" {
		o.Executable = DefaultExecutable
	}
	if o.Sandbox == "" {
		o.Sandbox = "workspace-write"
	}
	return &oneshot.Session{Executable: o.Executable, Dir: o.Dir, Dialect: dialect{o}}
}

type dialect struct{ o Options }

func (d dialect) Args(prompt, resume string) []string {
	args := []string{"exec", "--json", "--sandbox", d.o.Sandbox, "--skip-git-repo-check"}
	if d.o.Model != "" {
		args = append(args, "--model", d.o.Model)
	}
	if resume != "" {
		args = append(args, "resume", resume)
	}
	return append(args, prompt)
}

func (dialect) Decoder() oneshot.Decoder { return &decoder{} }

func (dialect) Missing() string {
	return "Install the Codex CLI (https://developers.openai.com/codex), sign in, then run `gofi doctor`."
}

// event is one line of `codex exec --json`.
type event struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Message  string `json:"message"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error"`
	Usage *struct {
		Input  int `json:"input_tokens"`
		Cached int `json:"cached_input_tokens"`
		Output int `json:"output_tokens"`
	} `json:"usage"`
	Item *item `json:"item"`
}

type item struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Command string          `json:"command"`
	Server  string          `json:"server"`
	Tool    string          `json:"tool"`
	Changes json.RawMessage `json:"changes"`
	Message string          `json:"message"`
}

type decoder struct {
	thread    string
	completed bool
	errs      []string
}

func (d *decoder) Resume() string { return d.thread }

func (d *decoder) Line(line []byte, emit func(engine.Event)) {
	var e event
	if json.Unmarshal(line, &e) != nil {
		return
	}
	switch e.Type {
	case "thread.started":
		d.thread = e.ThreadID
		emit(engine.Meta{SessionID: e.ThreadID})
	case "item.started":
		if b, ok := toolUse(e.Item); ok {
			emit(engine.Message{Blocks: []engine.Block{b}})
		}
	case "item.completed":
		if e.Item == nil {
			return
		}
		switch e.Item.Type {
		case "agent_message":
			emit(engine.Message{Blocks: []engine.Block{{Kind: engine.BlockText, Text: e.Item.Text}}})
		case "reasoning":
			emit(engine.Message{Blocks: []engine.Block{{Kind: engine.BlockThinking, Text: e.Item.Text}}})
		case "file_change":
			emit(engine.Message{Blocks: []engine.Block{{Kind: engine.BlockToolUse, ID: e.Item.ID, Name: "file_change", Input: e.Item.Changes}}})
		case "error":
			d.errs = append(d.errs, e.Item.Message)
		}
	case "turn.completed":
		d.completed = true
		if u := e.Usage; u != nil {
			emit(engine.Usage{Total: true, Input: u.Input, Output: u.Output, CacheRead: u.Cached})
		}
	case "turn.failed":
		if e.Error != nil {
			d.errs = append(d.errs, e.Error.Message)
		}
	case "error":
		d.errs = append(d.errs, e.Message)
	}
}

// toolUse is a command or an MCP call the model started, shown as it starts.
func toolUse(it *item) (engine.Block, bool) {
	if it == nil {
		return engine.Block{}, false
	}
	switch it.Type {
	case "command_execution":
		in, _ := json.Marshal(map[string]string{"command": it.Command})
		return engine.Block{Kind: engine.BlockToolUse, ID: it.ID, Name: "shell", Input: in}, true
	case "mcp_tool_call":
		return engine.Block{Kind: engine.BlockToolUse, ID: it.ID, Name: it.Server + "." + it.Tool, Input: json.RawMessage("{}")}, true
	}
	return engine.Block{}, false
}

func (d *decoder) End(emit func(engine.Event), failed error, stderr string, took time.Duration) {
	switch {
	case len(d.errs) > 0:
		emit(engine.Done{IsError: true, Err: strings.Join(d.errs, "; "), Duration: took})
	case d.completed && failed == nil:
		emit(engine.Done{Duration: took})
	case failed != nil && stderr != "":
		emit(engine.Done{IsError: true, Err: stderr, Duration: took})
	default:
		emit(engine.Failure{Message: "codex ended the turn without completing it", Hint: stderr})
	}
}
