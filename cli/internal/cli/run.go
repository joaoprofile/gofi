package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/engine/claude"
	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// sensorTimeout bounds one sensor stage: a test suite that hangs must not
// hold the run forever.
const sensorTimeout = 20 * time.Minute

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <change> | --resume <run>",
		Short: i18n.T("cmd.run.short"),
		Long: `Carry an approved change out on its own: the delivery flow (gofi flow check
delivery) implements it, checks it with the sensors, audits it and, accepted,
archives it into the living specs (decision 0002).

Only a sealed change runs (gofi change approve). Each agent turn is a fresh
session; the state is checkpointed after every node, so a run interrupted —
Ctrl+C, gofi cancel, a crash — resumes with --resume where it stopped. Every
transition goes to .gofi/runs/<run>/events.ndjson with why it was taken; each
agent's turn to agents/<node>-<iter>.ndjson.

A decision the change does not cover stops the implementation: in a terminal
the question is asked, in a script nobody answers and the run ends in impasse.
The sensors run the commands of loop.sensors in .gofi.yaml; test defaults to
the test section's default task, and a stage with no command is recorded as
not checked, never as passed.`,
		Example: `gofi run adicionar-rate-limit
gofi run --resume r-20261102-101500
gofi run adicionar-rate-limit --no-archive`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRun,
	}
	cmd.Flags().String("resume", "", i18n.T("cmd.run.flag.resume"))
	cmd.Flags().String("flow", "delivery", i18n.T("cmd.run.flag.flow"))
	cmd.Flags().Bool("no-archive", false, i18n.T("cmd.run.flag.no_archive"))
	cmd.Flags().String("permission-mode", "acceptEdits", "Claude Code permission mode for the agent turns")
	return cmd
}

// runPorts lets a test replace the world gofi run reaches: the engine
// sessions and the person at the gate.
var runPorts = struct {
	open func(root string) engine.Session
	gate loop.Gate
}{}

func runRun(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	resume, _ := cmd.Flags().GetString("resume")
	if (resume == "") == (len(args) == 0) {
		return errors.New(i18n.T("run.which"))
	}
	out := cmd.OutOrStdout()
	flowName, _ := cmd.Flags().GetString("flow")
	f, err := loop.Load(root, flowName)
	if err != nil {
		return err
	}
	for _, p := range loop.Check(f, nil) {
		if p.Error {
			return fmt.Errorf("%s: %s — gofi flow check %s", f.Source, p.Message, flowName)
		}
	}

	open := runPorts.open
	if open == nil {
		if err := requireClaudeCode(os.Getenv(EnvEngine)); err != nil {
			return err
		}
		mode, _ := cmd.Flags().GetString("permission-mode")
		open = func(root string) engine.Session {
			return claude.New(claude.Options{Executable: os.Getenv(EnvEngine), Dir: root, PermissionMode: mode,
				AllowedTools: append(append([]string{}, guard.ReadOnlyRules...), guard.WriteRules...)})
		}
	}
	gate := runPorts.gate
	if gate == nil {
		gate = termGate{interactive: term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))}
	}

	run, change, ctxName := resume, "", ""
	if resume == "" {
		change = args[0]
		ctxName, err = sealedChange(root, change)
		if err != nil {
			return err
		}
		run = "r-" + time.Now().Format("20060102-150405")
	} else {
		s, err := loop.LoadState(root, resume)
		if err != nil {
			return err
		}
		change, ctxName = s.Change, s.Context
	}

	release, err := loop.Acquire(root, run)
	if err != nil {
		return err
	}
	defer release()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := &loop.Runner{
		Root: root, Flow: f, Brief: loop.DefaultBrief,
		Agent:    &engineAgent{root: root, run: run, open: open, out: out},
		Sensors:  commandSensors{root: root, commands: sensorCommands(cfg)},
		Verdicts: diskVerdicts{root: root},
		Gate:     gate,
		OnEvent: func(e loop.Event) {
			switch e.Ev {
			case "node.start":
				fmt.Fprintf(out, "\n● %s\n", e.Node)
			case "edge":
				fmt.Fprintf(out, "  → %s  (%s)\n", e.To, e.Why)
			case "node.failed", "run.end", "run.cancelled":
				fmt.Fprintf(out, "  %s %s\n", e.Ev, e.Why)
			}
		},
	}
	fmt.Fprintln(out, i18n.T("run.started", run, change, f.Source))
	var s *loop.State
	if resume == "" {
		s, err = r.Start(ctx, run, change, ctxName)
	} else {
		s, err = r.Resume(ctx, run)
	}
	if err != nil {
		fmt.Fprintln(out, "\n  "+i18n.T("run.stopped", run, run))
		return &ExitCodeError{Code: 1, Err: err}
	}
	fmt.Fprintf(out, "\n  %s\n", i18n.T("run.ended", run, s.Outcome, s.CostUSD))
	if s.Outcome != "accepted" {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("run.not_accepted", s.Outcome))}
	}
	if noArchive, _ := cmd.Flags().GetBool("no-archive"); noArchive {
		return nil
	}
	all, err := contract.Docs(root)
	if err != nil {
		return err
	}
	res, err := contract.Archive(root, all, change, time.Now().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("%s — gofi change archive %s: %w", i18n.T("run.archive_failed"), change, err)
	}
	fmt.Fprintln(out, "  "+i18n.T("change.archive.moved", res.Dir))
	return nil
}

// sealedChange checks that a change is open and sealed, and returns the
// context its verdict is about: the first its proposal names.
func sealedChange(root, change string) (string, error) {
	all, err := contract.Docs(root)
	if err != nil {
		return "", err
	}
	for _, c := range contract.Changes(all) {
		if c.ID != change {
			continue
		}
		if !c.Sealed {
			return "", errors.New(i18n.T("run.not_sealed", change, change))
		}
		if len(c.Contexts) == 0 {
			return "", fmt.Errorf("a mudança %s não nomeia contexto", change)
		}
		return c.Contexts[0], nil
	}
	return "", errors.New(i18n.T("change.not_found", change))
}

// sensorCommands are the commands of each stage: loop.sensors, with test
// defaulting to the test section's default task.
func sensorCommands(cfg *config.GofiConfig) map[string]string {
	cmds := map[string]string{}
	if cfg.Loop != nil {
		for k, v := range cfg.Loop.Sensors {
			cmds[k] = v
		}
	}
	if _, ok := cmds["test"]; !ok {
		if t, ok := cfg.Test.Tasks[cfg.Test.Default]; ok && !strings.HasPrefix(strings.TrimSpace(t.Run), "echo ") {
			cmds["test"] = t.Run
		}
	}
	return cmds
}

// engineAgent runs a role's turn in a fresh engine session and keeps what
// the agent did in agents/<node>-<iter>.ndjson.
type engineAgent struct {
	root, run string
	open      func(root string) engine.Session
	out       io.Writer
}

func (a *engineAgent) Run(ctx context.Context, t loop.Turn) (loop.TurnResult, error) {
	s := a.open(a.root)
	defer s.Close()
	dir := filepath.Join(loop.Dir(a.root, a.run), "agents")
	_ = os.MkdirAll(dir, 0o755)
	transcript, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%d.ndjson", t.Node, t.Iter)))
	if err != nil {
		return loop.TurnResult{}, err
	}
	defer transcript.Close()
	record := func(kind, text string) {
		line, _ := json.Marshal(map[string]string{"t": time.Now().UTC().Format(time.RFC3339), "kind": kind, "text": text})
		_, _ = transcript.Write(append(line, '\n'))
	}
	record("prompt", t.Prompt)

	events, err := s.Send(ctx, engine.Turn{Prompt: t.Prompt})
	if err != nil {
		return loop.TurnResult{}, err
	}
	var res loop.TurnResult
	var failure error
	for e := range events {
		switch ev := e.(type) {
		case engine.Message:
			for _, b := range ev.Blocks {
				switch b.Kind {
				case engine.BlockText:
					record("text", b.Text)
				case engine.BlockToolUse:
					record("tool", b.Name+" "+string(b.Input))
					fmt.Fprintf(a.out, "  · %s\n", b.Name)
				}
			}
		case engine.Done:
			res = loop.TurnResult{CostUSD: ev.CostUSD, Seconds: ev.Duration.Seconds()}
			record("done", fmt.Sprintf("%.1fs $%.3f", ev.Duration.Seconds(), ev.CostUSD))
			if ev.IsError {
				failure = errors.New(ev.Err)
			} else if ev.Cancelled {
				failure = context.Canceled
			}
		case engine.Failure:
			failure = errors.New(ev.Message)
			record("failure", ev.Message)
		}
	}
	return res, failure
}

// commandSensors runs each stage's command from the project root, cheapest
// first, and stops at the first that fails.
type commandSensors struct {
	root     string
	commands map[string]string
}

func (c commandSensors) Run(ctx context.Context, stages []string) loop.SensorResult {
	var res loop.SensorResult
	var notes []string
	for _, stage := range stages {
		command := strings.TrimSpace(c.commands[stage])
		if command == "" {
			notes = append(notes, stage+": não verificado — sem comando em loop.sensors")
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, sensorTimeout)
		shell, flag := "sh", "-c"
		if runtime.GOOS == "windows" {
			shell, flag = "cmd", "/C"
		}
		cmd := exec.CommandContext(sctx, shell, flag, command)
		cmd.Dir = c.root
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			res.Failed = append(res.Failed, stage)
			res.Output = strings.Join(append(notes, stage+": "+command+"\n"+string(output)), "\n")
			return res
		}
	}
	res.Output = strings.Join(notes, "\n")
	return res
}

type diskVerdicts struct{ root string }

func (d diskVerdicts) Read(ctx string) (*verdict.Verdict, error) { return verdict.Read(d.root, ctx) }

// termGate asks the person at the terminal; in a script nobody is there.
type termGate struct{ interactive bool }

func (g termGate) Ask(_ context.Context, e *intake.Elicitation) (map[string]string, bool) {
	if !g.interactive {
		return nil, false
	}
	answers, err := askQuestions(e.AsQuestions())
	return answers, err == nil
}
