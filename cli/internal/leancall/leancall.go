// Package leancall asks a cheap model one closed question: a system prompt, a
// request, and the JSON Schema the answer must match — nothing of the project
// loaded, no tool, no session kept.
//
// It is the call measured in bench/results/B4-lean-call.md: about a fifth of
// the cost of a plain headless call, because the host's default context never
// loads. Claude Code's --bare would cut more, but it needs an API key and does
// not work with the subscription login most people use; this works with both.
package leancall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Request is one question to the model.
type Request struct {
	System string
	Prompt string
	// Schema is the JSON Schema the answer must match.
	Schema []byte
	// Model is the host's name for it (an alias such as haiku, or an ID).
	Model string
	// MaxUSD caps what the call may spend.
	MaxUSD float64
	// Timeout bounds the call.
	Timeout time.Duration
}

// Spend is what a call cost, as the host reports it.
type Spend struct {
	USD          float64 `json:"usd"`
	Millis       int     `json:"ms"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
}

// Claude asks through Claude Code in print mode.
type Claude struct {
	// Executable is the claude binary; empty means "claude" on PATH.
	Executable string
}

// Call runs the request and returns the validated answer.
//
// The call runs in an empty temporary directory, so no project file —
// AGENTS.md, CLAUDE.md, settings — is found and loaded, and with every other
// source of context turned off by flag. A reply that is not a validated
// structured answer is an error: the caller falls back to its own rules.
func (c Claude) Call(ctx context.Context, r Request) ([]byte, Spend, error) {
	exe := c.Executable
	if exe == "" {
		exe = "claude"
	}
	if r.Timeout == 0 {
		r.Timeout = 60 * time.Second
	}
	if r.MaxUSD == 0 {
		r.MaxUSD = 0.05
	}
	dir, err := os.MkdirTemp("", "gofi-leancall-")
	if err != nil {
		return nil, Spend{}, err
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, Args(r)...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, Spend{}, fmt.Errorf("claude: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return parse(out.Bytes())
}

// Args are the flags of the lean call. Exported so the measurement and the
// code cannot drift apart.
func Args(r Request) []string {
	return []string{
		"-p", r.Prompt,
		"--model", r.Model,
		// A closed question over given options needs no long reasoning, and
		// the reasoning is output — the dearer half of the bill (bench C/D).
		"--effort", "low",
		"--tools", "",
		"--system-prompt", r.System,
		"--disable-slash-commands",
		"--strict-mcp-config",
		"--setting-sources", "project",
		"--exclude-dynamic-system-prompt-sections",
		"--no-session-persistence",
		"--output-format", "json",
		"--json-schema", string(r.Schema),
		"--max-budget-usd", strconv.FormatFloat(r.MaxUSD, 'f', -1, 64),
	}
}

// reply is the part of Claude Code's JSON output the call reads.
type reply struct {
	Subtype    string          `json:"subtype"`
	IsError    bool            `json:"is_error"`
	Structured json.RawMessage `json:"structured_output"`
	CostUSD    float64         `json:"total_cost_usd"`
	DurationMS int             `json:"duration_ms"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func parse(b []byte) ([]byte, Spend, error) {
	var r reply
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, Spend{}, fmt.Errorf("claude: unreadable reply: %w", err)
	}
	spend := Spend{USD: r.CostUSD, Millis: r.DurationMS, InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens}
	if r.IsError || r.Subtype != "success" {
		return nil, spend, fmt.Errorf("claude: %s", r.Subtype)
	}
	if len(r.Structured) == 0 || string(r.Structured) == "null" {
		return nil, spend, errors.New("claude: no structured answer")
	}
	return r.Structured, spend, nil
}
