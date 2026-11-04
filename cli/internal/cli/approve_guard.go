package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

// agentEnv are the variables a coding agent sets in the shell it runs
// commands in. A command run there is the agent's, not the person's.
var agentEnv = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED", "CURSOR_AGENT", EnvCodex, EnvCopilot}

// approvePorts are what personApproves asks the machine: whether an agent
// runs the command, and whether a person is at the terminal.
var approvePorts = struct {
	agent func() bool
	tty   func() bool
}{
	agent: func() bool {
		for _, v := range agentEnv {
			if os.Getenv(v) != "" {
				return true
			}
		}
		return false
	},
	tty: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
}

// personApproves stands between the approve commands and the seal: approving
// is the person's, so a command an agent runs is refused, and at a terminal
// the person confirms what is sealed. Without a terminal and without an agent
// — the editor's panel after the person clicked, a script — it goes on.
func personApproves(in io.Reader, out io.Writer, docs []contract.Doc) error {
	if approvePorts.agent() {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("approve.agent"))}
	}
	if !approvePorts.tty() {
		return nil
	}
	fmt.Fprintln(out, "  "+i18n.T("approve.confirm.list"))
	for _, d := range docs {
		fmt.Fprintln(out, "    "+d.Path)
	}
	fmt.Fprint(out, "  "+i18n.T("approve.confirm.ask")+" ")
	answer, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "s", "sim", "y", "yes":
		return nil
	}
	return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("approve.declined"))}
}
