package aiclient

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

// claudeClient drives the `claude` CLI shipped with Claude Code. The prompt
// is fed via stdin in --print mode, so it can be arbitrarily long without
// hitting argv limits.
type claudeClient struct {
	bin string // overridable for tests
}

func newClaudeClient() *claudeClient {
	return &claudeClient{bin: "claude"}
}

func (c *claudeClient) Name() string { return "claude-vscode" }

func (c *claudeClient) Available() bool {
	if _, err := exec.LookPath(c.bin); err == nil {
		return true
	}
	return false
}

func (c *claudeClient) Invoke(ctx context.Context, prompt string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, c.bin, "--print")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
