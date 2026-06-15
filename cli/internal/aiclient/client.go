// Package aiclient invokes the CLI of the AI host configured in .gofi.yaml.
// Used by `gofi train` to ask the active agent to read and acknowledge new
// training content.
package aiclient

import (
	"context"
	"fmt"
	"io"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// Client is the contract every AI-host adapter implements. New hosts
// (cursor, windsurf, etc.) plug in by satisfying this interface and
// registering themselves in ForHost.
type Client interface {
	// Name returns the slug stored in .gofi.yaml under ai.host.
	Name() string

	// Available reports whether the underlying CLI is reachable on PATH.
	// Callers can short-circuit when false instead of failing.
	Available() bool

	// Invoke runs the host CLI with prompt as its input. stdout/stderr
	// receive the host's output; ctx allows cancellation.
	Invoke(ctx context.Context, prompt string, stdout, stderr io.Writer) error
}

// ForHost returns the Client implementation for the AI host slug. Returns
// an error when the slug is unknown.
func ForHost(host string) (Client, error) {
	switch host {
	case config.AIHostClaudeVSCode:
		return newClaudeClient(), nil
	default:
		return nil, fmt.Errorf("unknown ai host %q (only %q is supported in v1)", host, config.AIHostClaudeVSCode)
	}
}
