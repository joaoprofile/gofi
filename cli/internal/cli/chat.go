package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/approval"
	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/engine/claude"
	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/toolchain"
	"github.com/joaoprofile/gofi/cli/internal/tui/chat"
)

// EnvEngine overrides the Claude Code binary the chat drives.
const EnvEngine = "GOFI_CLAUDE"

// permissionAsk is the chat's own mode: every change is approved in the chat.
const permissionAsk = "ask"

type chatFlags struct {
	model          string
	resume         string
	permissionMode string
}

func newChatCmd() *cobra.Command {
	var f chatFlags
	cmd := &cobra.Command{
		Use:   "chat",
		Short: i18n.T("cmd.chat.short"),
		Long: `Talk to the agents of this project in the terminal.

Type a request, run a skill (/gofi-pd, /gofi-eng, …) or a shell command
(!git status). The conversation runs on Claude Code in the project root, so
every skill, memory and knowledge file the project installed is available.

In a project, a request in free text is planned first, as 'gofi intake' does:
the plan is shown, its questions asked (answer with the option's number, or
"seguir"), and its phases run one turn each, each role at its tier. The chat
stops after a PRD or a spec for review. /brief shows the assembled request;
/send <text> sends text as typed, with no plan.

The prompt takes as many lines as the request needs: alt+enter (or ctrl+j, or
a backslash before enter) breaks the line, and a pasted text keeps its own.
Typing / lists every command — the chat's, the project's skills and the
engine's, each with what it does — and @ completes a file to point at.
ctrl+v attaches the picture in the clipboard (alt+v where the terminal keeps
ctrl+v for itself).

While the agent works, enter sends a follow-up: the engine hands it to the
model at its next step — between two tool calls — and the turn goes on knowing
it. It costs the message and nothing else. Esc stops the step under way.

The top of the screen is the panel of what is at work, as it happens: the plan
being conducted and its phases, this conversation's turn — how long, how many
tools, the last one —, the subagents it started, and the runs of the loop with
their agents, started here, by /gofi-run or in another terminal. It is read
from what the engine streams and from the runs on disk; no model is called to
draw it. /watch opens the full screen. On a terminal under 18 lines the panel
shows over the prompt only while something is at work.

/find, /show, /impact and /path ask the project's index and /runs lists the
runs under way: gofi reads the disk and no model is called.

Before the agent edits a file or runs a command, the chat shows what it is —
the command, the change as a diff — and asks: yes; yes and do not ask again
for commands that begin the same way, or for file edits, until the
conversation ends; or no, with what the agent should do instead. Reading and
searching never ask. --permission-mode hands the decision to Claude Code
instead (acceptEdits, plan, bypassPermissions, …).`,
		Example: `gofi chat
gofi chat --model sonnet
gofi chat --resume <session-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChat(f)
		},
	}
	cmd.Flags().StringVar(&f.model, "model", "", "model for this conversation (default: ai.model in .gofi.yaml)")
	cmd.Flags().StringVar(&f.resume, "resume", "", "continue a Claude Code conversation by id")
	cmd.Flags().StringVar(&f.permissionMode, "permission-mode", permissionAsk, "ask (the chat approves each change) or a Claude Code mode: acceptEdits | plan | bypassPermissions | auto")
	return cmd
}

func runChat(f chatFlags) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New(i18n.T("chat.no_tty"))
	}
	if h := projectHost(); !h.Chat {
		return fmt.Errorf("%s", i18n.T("chat.host_required", h.Label, h.Label))
	}
	if err := requireClaudeCode(os.Getenv(EnvEngine)); err != nil {
		return err
	}
	// Outside a project the chat still works — it just has no skills to offer.
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	model := strings.TrimSpace(f.model)
	var planIntake func(string, map[string]string, bool) (*intake.Result, error)
	var tierModel func(host.Tier) string
	var review func(context.Context, string, func(string)) ([]string, int, error)
	if cfg, projectRoot, err := loadProjectConfig(); err == nil {
		root = projectRoot
		review = chatReview(projectRoot, cfg)
		if model == "" {
			model = cfg.AI.Model
		}
		planIntake = chatIntake(cfg, projectRoot)
		if h, ok := host.Get(cfg.AI.Host); ok && h.SkillModel {
			tierModel = func(t host.Tier) string { return h.Model(t, cfg.AI.Tiers) }
		}
	} else if !errors.Is(err, ErrNotInProject) {
		return err
	}

	// In the chat's own mode, gofi is the engine's PreToolUse hook and the
	// engine runs in `auto`, so what the user approved here is carried out
	// without a second dialog of the engine's own.
	mode, settings := strings.TrimSpace(f.permissionMode), ""
	var approvals *approval.Server
	if mode == "" || mode == permissionAsk {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if approvals, err = approval.Listen(); err != nil {
			return fmt.Errorf("open the approval channel: %w", err)
		}
		defer approvals.Close()
		if settings, err = approval.HookSettings(exe, approvals.Addr()); err != nil {
			return err
		}
		mode = "auto"
	}

	// The chat asks gofi itself what the index knows and opens its watch
	// screen: the same binary, in the project.
	var gofi func(args ...string) *exec.Cmd
	if self, err := os.Executable(); err == nil {
		gofi = func(args ...string) *exec.Cmd {
			cmd := exec.Command(self, args...)
			cmd.Dir = root
			return cmd
		}
	}

	resume := strings.TrimSpace(f.resume)
	return chat.Run(chat.Options{
		Root:      root,
		Version:   Version,
		Model:     model,
		Approvals: approvals,
		Sessions:  func() []engine.Saved { return claude.ListSessions(root, 10) },
		Models:    config.AllModels(),
		Intake:    planIntake,
		TierModel: tierModel,
		Deliver:   spawnDelivery(root),
		Review:    review,
		Gofi:      gofi,
		Inset:     stickyRows(),
		NewSession: func(resumeID string) engine.Session {
			if resumeID != "" {
				resume = resumeID
			}
			s := claude.New(claude.Options{
				Executable:     os.Getenv(EnvEngine),
				Dir:            root,
				Model:          model,
				ResumeID:       resume,
				PermissionMode: mode,
				Settings:       settings,
				AllowedTools:   guard.WriteRules,
				// A conversation opens several sessions — its own, one per
				// phase of a plan, another after /clear: with what changes
				// between them out of the system prompt, each reads that
				// prefix from the cache the one before it wrote.
				StablePrompt: true,
				// Esc stops the answer, not the engine: the next message
				// goes to the same process.
				Interrupt: true,
			})
			resume = "" // consumed: /clear starts afresh, not back where it pointed
			return s
		},
	})
}

// stickyRows is how many rows at the top of the screen the terminal keeps
// for itself. VS Code's terminal pins the command that is running over the
// first row once its prompt has scrolled away; whatever is drawn there is
// under it.
func stickyRows() int {
	if os.Getenv("TERM_PROGRAM") == "vscode" {
		return 1
	}
	return 0
}

// chatIntake plans the chat's requests the way `gofi intake` does: the light
// tier for what the rules leave open, and its decisions cached.
func chatIntake(cfg *config.GofiConfig, root string) func(string, map[string]string, bool) (*intake.Result, error) {
	return func(text string, answers map[string]string, proceed bool) (*intake.Result, error) {
		return intake.Run(root, text, intake.Options{
			Language: backendLanguage(cfg),
			Model:    lightModelFor(cfg),
			CacheDir: filepath.Join(root, ".gofi", "cache", "intake"),
			Answers:  answers,
			Proceed:  proceed,
		})
	}
}

// requireClaudeCode refuses to start a conversation on a Claude Code older
// than the minimum: it would run without the project's AGENTS.md, and nothing
// on screen would say so.
func requireClaudeCode(executable string) error {
	if c := toolchain.ClaudeCode(executable); !c.OK {
		return fmt.Errorf("%s", i18n.T("chat.claude_required", toolchain.MinClaudeCode, c.Hint))
	}
	return nil
}
