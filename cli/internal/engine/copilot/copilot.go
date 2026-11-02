// Package copilot runs the GitHub Copilot CLI as an engine.Session: one
// `copilot -p` per turn, its response read as text. Each turn is a
// conversation of its own — what a turn needs from the one before is on
// disk, as gofi run already arranges it.
package copilot

import (
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/engine/oneshot"
)

// DefaultExecutable is the CLI looked up on PATH.
const DefaultExecutable = "copilot"

// Options configures a session.
type Options struct {
	Executable string // DefaultExecutable when empty
	Dir        string // the project root the engine runs in
	Model      string // passed as --model when set
}

// New opens a session; nothing runs until the first turn.
func New(o Options) *oneshot.Session {
	if o.Executable == "" {
		o.Executable = DefaultExecutable
	}
	return &oneshot.Session{Executable: o.Executable, Dir: o.Dir, Dialect: dialect{o}}
}

type dialect struct{ o Options }

// Args run the prompt headless: every tool allowed, as the CLI requires
// without a person to approve, never pausing to ask, only the response on
// stdout.
func (d dialect) Args(prompt, _ string) []string {
	args := []string{"-p", prompt, "--allow-all-tools", "--no-ask-user", "--silent"}
	if d.o.Model != "" {
		args = append(args, "--model", d.o.Model)
	}
	return args
}

func (dialect) Decoder() oneshot.Decoder { return &decoder{} }

func (dialect) Missing() string {
	return "Install the GitHub Copilot CLI (https://docs.github.com/copilot/how-tos/set-up/install-copilot-cli), sign in, then run `gofi doctor`."
}

type decoder struct{ text strings.Builder }

func (*decoder) Resume() string { return "" }

func (d *decoder) Line(line []byte, _ func(engine.Event)) {
	d.text.Write(line)
	d.text.WriteByte('\n')
}

func (d *decoder) End(emit func(engine.Event), failed error, stderr string, took time.Duration) {
	text := strings.TrimSpace(d.text.String())
	if text != "" {
		emit(engine.Message{Blocks: []engine.Block{{Kind: engine.BlockText, Text: text}}})
	}
	switch {
	case failed != nil:
		msg := stderr
		if msg == "" {
			msg = failed.Error()
		}
		emit(engine.Done{IsError: true, Err: msg, Duration: took})
	case text == "":
		emit(engine.Failure{Message: "copilot ended the turn without a response", Hint: stderr})
	default:
		emit(engine.Done{Duration: took})
	}
}
