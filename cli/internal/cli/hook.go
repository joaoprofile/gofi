package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/gofi-labs/gofi/cli/internal/approval"
)

// newHookCmd holds the commands the engine runs as hooks. They are gofi's side
// of a protocol, not something to type, so they stay out of the help.
func newHookCmd() *cobra.Command {
	hook := &cobra.Command{
		Use:    "hook",
		Short:  "Engine hooks (run by Claude Code, not by hand)",
		Hidden: true,
	}
	var addr string
	pretool := &cobra.Command{
		Use:   "pretool",
		Short: "PreToolUse hook: ask the gofi chat before a tool changes something",
		Long: `Claude Code runs this before a tool that can change something (Edit,
Write, Bash, …) while gofi chat is open. It reads the tool call from stdin,
asks the chat over its local socket, and prints the decision for the engine:
"allow" runs the call, "deny" blocks it and tells the model why.

It fails closed: no chat, no answer or an unreadable one all deny.
gofi chat installs it through --settings; there is no reason to run it by hand.`,
		Example: `echo '{"tool_name":"Bash","tool_input":{"command":"ls"}}' | gofi hook pretool --addr /tmp/gofi-approve-1/s`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, _ := io.ReadAll(os.Stdin)
			_, err := os.Stdout.Write(append(approval.HookResponse(approval.Ask(in, addr)), '\n'))
			return err
		},
	}
	pretool.Flags().StringVar(&addr, "addr", "", "the chat's approval socket")
	guardHook := &cobra.Command{
		Use:   "guard",
		Short: "PreToolUse and UserPromptSubmit hook: ask the index before searching the tree",
		Long: `Claude Code runs this on every prompt and before Grep, Glob, Read, Bash and the
gofi MCP tools. It reads the hook input from stdin and, in the mode ai.guard
names, reminds or blocks an agent that searches before asking the index.
'gofi guard' installs it; there is no reason to run it by hand.`,
		Example: `echo '{"session_id":"s","hook_event_name":"PreToolUse","tool_name":"Grep","tool_input":{"pattern":"Bill"}}' | gofi hook guard`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runHookGuard(os.Stdin, os.Stdout)
			return nil
		},
	}
	intakeHook := &cobra.Command{
		Use:   "intake",
		Short: "UserPromptSubmit hook: hand the agent the plan of a request",
		Long: `Claude Code runs this on every prompt when ai.intake is hint. It plans the
prompt with the rules — no model, no token — and hands the agent the plan, the
open questions and the evidence to read as context. The session's model is
left alone. Quiet on a command, outside a project, while gofi chat or gofi
ask conduct a plan, and on any error of its own.
'gofi install guard' installs it; there is no reason to run it by hand.`,
		Example: `echo '{"hook_event_name":"UserPromptSubmit","prompt":"altere o fluxo de pricing","cwd":"."}' | gofi hook intake`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runHookIntake(os.Stdin, os.Stdout)
			return nil
		},
	}
	hook.AddCommand(pretool, guardHook, intakeHook)
	return hook
}
