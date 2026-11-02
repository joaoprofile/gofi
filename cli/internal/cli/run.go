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
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
	"github.com/joaoprofile/gofi/cli/internal/worktree"
)

// sensorTimeout bounds one sensor stage: a test suite that hangs must not
// hold the run forever.
const sensorTimeout = 20 * time.Minute

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <change> | --context <ctx> | --resume <run>",
		Short: i18n.T("cmd.run.short"),
		Long: `Carry an approved change out on its own: the delivery flow (gofi flow check
delivery) implements it, checks it with the sensors, audits it and, accepted,
archives it into the living specs (decision 0002).

Only a sealed change runs (gofi change approve). The agents work on the
project's host — ai.host, as gofi init set it: Claude Code, Codex or Copilot.
Each agent turn is a fresh session; the state is checkpointed after every node, so a run interrupted —
Ctrl+C, gofi cancel, a crash — resumes with --resume where it stopped. Every
transition goes to .gofi/runs/<run>/events.ndjson with why it was taken; each
agent's turn to agents/<node>-<iter>.ndjson.

--context delivers every sealed change of a context, in the order their
dependencies allow, and stops at the first not accepted.

A decision the change does not cover stops the implementation at the gate:
the question goes to .gofi/runs/<run>/gate.json and is answered with
gofi answer <run> from any terminal — or right there, when the run has one.
The run waits for --gate-timeout (loop.gate_timeout, 30m by default; 0 does
not wait) and, unanswered, ends in impasse.

--detach carries the run out in the background and prints its id: follow it
with gofi watch or gofi logs -f; what the supervisor prints goes to
.gofi/runs/<run>/supervisor.log.

--worktree — always the case with --detach — runs it in a git worktree of its
own, on the branch gofi/<change>, so several changes run at the same time.
The change must be committed. Accepted, the branch is committed, rebased on
the current branch once every change it depends on has landed, and
fast-forwarded into it — only while the project's tree is clean, waiting up
to --gate-timeout otherwise; never merged, never pushed. A conflict ends in
"conflito", the branch kept for the person.
The sensors run the commands of loop.sensors in .gofi.yaml; test defaults to
the test section's default task, and a stage with no command is recorded as
not checked, never as passed.`,
		Example: `gofi run adicionar-rate-limit
gofi run --context pedidos
gofi run --resume r-20261102-101500
gofi run adicionar-rate-limit --no-archive
gofi run adicionar-rate-limit --detach`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRun,
	}
	cmd.Flags().String("resume", "", i18n.T("cmd.run.flag.resume"))
	cmd.Flags().String("context", "", i18n.T("cmd.run.flag.context"))
	cmd.Flags().String("flow", "delivery", i18n.T("cmd.run.flag.flow"))
	cmd.Flags().Bool("no-archive", false, i18n.T("cmd.run.flag.no_archive"))
	cmd.Flags().String("permission-mode", "acceptEdits", "Claude Code permission mode for the agent turns")
	cmd.Flags().Bool("events", false, i18n.T("cmd.run.flag.events"))
	cmd.Flags().String("gate-timeout", "", i18n.T("cmd.run.flag.gate_timeout"))
	cmd.Flags().Bool("detach", false, i18n.T("cmd.run.flag.detach"))
	cmd.Flags().Bool("worktree", false, i18n.T("cmd.run.flag.worktree"))
	cmd.Flags().String("run-id", "", "name of the new run (set by --detach)")
	_ = cmd.Flags().MarkHidden("run-id")
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
	ctxName, _ := cmd.Flags().GetString("context")
	named := 0
	for _, set := range []bool{resume != "", len(args) > 0, ctxName != ""} {
		if set {
			named++
		}
	}
	if named != 1 {
		return errors.New(i18n.T("run.which"))
	}
	if detach, _ := cmd.Flags().GetBool("detach"); detach {
		if ctxName != "" {
			return errors.New(i18n.T("run.detach_which"))
		}
		pass := args
		if resume != "" {
			pass = []string{"--resume", resume}
		}
		for _, f := range []string{"flow", "permission-mode", "gate-timeout"} {
			if cmd.Flags().Changed(f) {
				v, _ := cmd.Flags().GetString(f)
				pass = append(pass, "--"+f, v)
			}
		}
		if na, _ := cmd.Flags().GetBool("no-archive"); na {
			pass = append(pass, "--no-archive")
		}
		run, err := detachRun(root, pass...)
		if err != nil {
			return &ExitCodeError{Code: 1, Err: err}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "  "+i18n.T("run.detached", run, run))
		return nil
	}
	o := deliverOpts{Resume: resume}
	if len(args) > 0 {
		o.Change = args[0]
	}
	o.RunID, _ = cmd.Flags().GetString("run-id")
	o.Worktree, _ = cmd.Flags().GetBool("worktree")
	o.Flow, _ = cmd.Flags().GetString("flow")
	o.NoArchive, _ = cmd.Flags().GetBool("no-archive")
	o.Events, _ = cmd.Flags().GetBool("events")
	o.Mode, _ = cmd.Flags().GetString("permission-mode")
	o.GateTimeout, _ = cmd.Flags().GetString("gate-timeout")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var outcome string
	if ctxName != "" {
		outcome, err = deliverContext(ctx, cmd.OutOrStdout(), root, cfg, ctxName, o)
	} else {
		outcome, err = deliver(ctx, cmd.OutOrStdout(), root, cfg, o)
	}
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	if outcome != "accepted" {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("run.not_accepted", outcome))}
	}
	return nil
}

// deliverOpts is one delivery: a change to run, or a run to resume.
type deliverOpts struct {
	Change, Resume, Flow, Mode string
	NoArchive                  bool
	// Events writes the run's events as NDJSON, one per line, ending with a
	// run.result line — what the panel and the chat read.
	Events bool
	// GateTimeout overrides loop.gate_timeout (a duration; 0 does not wait).
	GateTimeout string
	// Worktree carries the run out in a worktree of its own, on the branch
	// gofi/<change>, and lands it on the current branch once accepted.
	Worktree bool
	// RunID names a new run; empty, one is made. A detached run is named by
	// the process that started it, so it can say which run it started.
	RunID string
}

// agentsCallThisGofi puts the folder of the running gofi first on the PATH
// the agents inherit: the commands a turn is told to run — gofi verdict
// write, gofi index — must be this version's, not an older gofi installed
// elsewhere on the PATH.
func agentsCallThisGofi() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dir := filepath.Dir(exe)
	path := os.Getenv("PATH")
	if first, _, _ := strings.Cut(path, string(os.PathListSeparator)); first == dir {
		return
	}
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}

// newRunID names a new run by the time it starts, made unique when another
// started in the same second.
func newRunID(root string) string {
	base := "r-" + time.Now().Format("20060102-150405")
	id := base
	for n := 2; ; n++ {
		if _, err := os.Stat(loop.Dir(root, id)); os.IsNotExist(err) {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// SupervisorLog is where a detached supervisor writes what it would have
// printed: in the run's folder, beside its events.
const SupervisorLog = "supervisor.log"

// detachRun starts gofi run in the background — a change, or --resume <run> —
// and returns the run it carries out. What the run does is on disk (gofi ps,
// gofi logs, gofi watch); what the supervisor prints goes to its log. The
// change is checked here, so a change that cannot run fails in the
// foreground, not in a log nobody reads.
func detachRun(root string, args ...string) (string, error) {
	var run string
	switch {
	case len(args) == 2 && args[0] == "--resume":
		run = args[1]
		s, err := loop.LoadState(root, run)
		if err != nil {
			return "", err
		}
		if s.Node == "" {
			return "", fmt.Errorf("run %s ended (%s)", run, s.Outcome)
		}
		if l, ok := loop.ReadLock(root, run); ok && loop.Alive(l.PID) {
			return "", fmt.Errorf("a execução %s já está rodando (pid %d)", run, l.PID)
		}
	case len(args) >= 1 && !strings.HasPrefix(args[0], "-"):
		if _, err := sealedChange(root, args[0]); err != nil {
			return "", err
		}
		// In the background, every run has a worktree of its own: two of
		// them in the same tree would write over each other.
		if err := worktree.Ready(context.Background(), root, args[0]); err != nil {
			return "", err
		}
		run = newRunID(root)
		args = append(args, "--run-id", run, "--worktree")
	default:
		return "", errors.New(i18n.T("run.detach_which"))
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := loop.Dir(root, run)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	log, err := os.OpenFile(filepath.Join(dir, SupervisorLog), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer log.Close()
	cmd := exec.Command(exe, append([]string{"run"}, args...)...)
	cmd.Dir, cmd.Stdout, cmd.Stderr, cmd.SysProcAttr = root, log, log, detached()
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return run, cmd.Process.Release()
}

// deliver carries one approved change out through a flow and, accepted,
// archives it. It is gofi run, and what every conductor hands a sealed
// change to: one executor for all of them.
func deliver(ctx context.Context, out io.Writer, root string, cfg *config.GofiConfig, o deliverOpts) (string, error) {
	if o.Flow == "" {
		o.Flow = "delivery"
	}
	f, err := loop.Load(root, o.Flow)
	if err != nil {
		return "", err
	}
	for _, p := range loop.Check(f, nil) {
		if p.Error {
			return "", fmt.Errorf("%s: %s — gofi flow check %s", f.Source, p.Message, o.Flow)
		}
	}
	agentsCallThisGofi()
	wait, err := gateTimeout(cfg, o.GateTimeout)
	if err != nil {
		return "", err
	}

	run, change, ctxName := o.Resume, o.Change, ""
	var prior *loop.State
	if o.Resume == "" {
		if ctxName, err = sealedChange(root, change); err != nil {
			return "", err
		}
		if run = o.RunID; run == "" {
			run = newRunID(root)
		}
	} else {
		if prior, err = loop.LoadState(root, o.Resume); err != nil {
			return "", err
		}
		change, ctxName = prior.Change, prior.Context
	}
	release, err := loop.Acquire(root, run)
	if err != nil {
		return "", err
	}
	defer release()

	// In a worktree the agents, the sensors, the verdict and the archive work
	// on the run's own tree and branch; the state stays at the root, where
	// gofi ps and gofi watch read every run.
	var tree *worktree.Tree
	switch {
	case prior != nil && prior.Worktree != "":
		tree = &worktree.Tree{Root: root, Dir: prior.Worktree, Branch: prior.Branch, Base: prior.Base}
	case prior == nil && o.Worktree:
		if tree, err = worktree.Prepare(ctx, root, run, change); err != nil {
			return "", err
		}
	}
	work := root
	if tree != nil {
		work = tree.Dir
	}
	report := humanReport(out)
	if o.Events {
		report = eventReport(out)
	}
	// Accepted and archived, a run in a worktree that has not landed yet
	// only lands: resuming it runs no agent again.
	if prior != nil && prior.Node == "" && prior.Outcome == "accepted" && tree != nil && prior.Landing != landed {
		return landRun(ctx, out, root, prior, tree, wait, o.Events, report)
	}

	open := runPorts.open
	if open == nil {
		mode := o.Mode
		if mode == "" {
			mode = "acceptEdits"
		}
		// Every agent of the run works on the project's host (ai.host).
		session, err := agentEngine(cfg, work, mode, append(append([]string{}, guard.ReadOnlyRules...), guard.WriteRules...))
		if err != nil {
			return "", err
		}
		open = func(string) engine.Session { return session("") }
	}
	gate := runPorts.gate
	if gate == nil {
		g := &loop.FileGate{Root: root, Run: run, Timeout: wait}
		if !o.Events && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
			g.Local = askAtTerminal(out, run)
		}
		gate = g
	}

	r := &loop.Runner{
		Root: root, Work: work, Flow: f, Brief: loop.DefaultBrief,
		Agent:    &engineAgent{root: root, work: work, run: run, open: open, out: out, quiet: o.Events},
		Sensors:  commandSensors{root: work, commands: sensorCommands(cfg)},
		Verdicts: diskVerdicts{root: work},
		Gate:     gate,
		OnEvent:  report,
	}
	if !o.Events {
		fmt.Fprintln(out, i18n.T("run.started", run, change, f.Source))
	}
	var s *loop.State
	if o.Resume == "" {
		if tree != nil {
			s, err = r.StartIn(ctx, run, change, ctxName, func(s *loop.State) {
				s.Worktree, s.Branch, s.Base = tree.Dir, tree.Branch, tree.Base
			})
		} else {
			s, err = r.Start(ctx, run, change, ctxName)
		}
	} else {
		s, err = r.Resume(ctx, run)
	}
	result := loop.Event{Ev: "run.result", Node: run}
	defer func() {
		if o.Events {
			report(result)
		}
	}()
	if err != nil {
		result.Why = "stopped: " + err.Error()
		if !o.Events {
			fmt.Fprintln(out, "\n  "+i18n.T("run.stopped", run, run))
		}
		return "", err
	}
	result.Why = s.Outcome
	if !o.Events {
		fmt.Fprintf(out, "\n  %s\n", i18n.T("run.ended", run, s.Outcome, s.CostUSD))
	}
	if s.Outcome != "accepted" || o.NoArchive {
		return s.Outcome, nil
	}
	all, err := contract.Docs(work)
	if err != nil {
		return s.Outcome, err
	}
	res, err := contract.Archive(work, all, change, time.Now().Format("2006-01-02"))
	if err != nil {
		return s.Outcome, fmt.Errorf("%s — gofi change archive %s: %w", i18n.T("run.archive_failed"), change, err)
	}
	result.To = res.Dir
	if !o.Events {
		fmt.Fprintln(out, "  "+i18n.T("change.archive.moved", res.Dir))
	}
	if tree == nil {
		return s.Outcome, nil
	}
	if err := tree.Commit(ctx, fmt.Sprintf("feat(%s): %s", ctxName, change)); err != nil {
		return s.Outcome, fmt.Errorf("commit %s: %w", tree.Branch, err)
	}
	outcome, err := landRun(ctx, out, root, s, tree, wait, o.Events, report)
	result.Why = outcome
	return outcome, err
}

// How a run in a worktree ended on the base branch.
const (
	landed   = "integrada"
	waiting  = "aguardando"
	conflict = "conflito"
)

// landPoll is how often a waiting landing looks again.
var landPoll = 10 * time.Second

// landRun lands an accepted run's branch on the base: once every change it
// depends on has landed, and while the project's tree is clean — waiting up
// to wait otherwise, the person's work never touched. Landed, the worktree
// goes; a conflict keeps the branch for the person. The outcome is accepted
// only when it landed.
func landRun(ctx context.Context, out io.Writer, root string, s *loop.State, tree *worktree.Tree, wait time.Duration, events bool, report func(loop.Event)) (string, error) {
	say := func(ev, why string) {
		report(loop.Event{Ev: ev, Node: s.Run, Why: why})
	}
	var deps []string
	if all, err := contract.Docs(tree.Dir); err == nil {
		for _, c := range contract.Changes(all) {
			if c.ID == s.Change {
				deps = c.DependsOn
			}
		}
	}
	deadline := time.Now().Add(wait)
	for {
		var why string
		pending := slices.IndexFunc(deps, func(d string) bool { return !worktree.Landed(ctx, root, tree.Base, d) })
		err := error(nil)
		if pending >= 0 {
			why = i18n.T("run.land.depends", deps[pending])
		} else if err = tree.Land(ctx); err == nil {
			s.Landing = landed
			_ = loop.SaveState(root, s)
			say("land.done", tree.Base)
			if !events {
				fmt.Fprintln(out, "  "+i18n.T("run.land.done", tree.Branch, tree.Base))
			}
			_ = tree.Remove(ctx)
			return "accepted", nil
		} else if errors.Is(err, worktree.ErrConflict) {
			s.Landing = conflict
			_ = loop.SaveState(root, s)
			say("land.conflict", err.Error())
			return conflict, errors.New(i18n.T("run.land.conflict", tree.Branch, tree.Base, tree.Dir))
		} else if errors.Is(err, worktree.ErrDirty) || errors.Is(err, worktree.ErrMoved) {
			why = err.Error()
		} else {
			return waiting, err
		}
		if s.Landing != waiting {
			s.Landing = waiting
			_ = loop.SaveState(root, s)
			say("land.waiting", why)
			if !events {
				fmt.Fprintln(out, "  "+i18n.T("run.land.waiting", tree.Branch, why))
			}
		}
		if time.Now().After(deadline) {
			return waiting, errors.New(i18n.T("run.land.gave_up", tree.Branch, s.Run))
		}
		select {
		case <-ctx.Done():
			return waiting, ctx.Err()
		case <-time.After(landPoll):
		}
	}
}

// deliverContext delivers every sealed change of a context, in the order
// their dependencies allow, and stops at the first not accepted. It is what
// a conductor hands the delivery to: the changes and their order are decided
// here, once.
func deliverContext(ctx context.Context, out io.Writer, root string, cfg *config.GofiConfig, ctxName string, o deliverOpts) (string, error) {
	all, err := contract.Docs(root)
	if err != nil {
		return "", err
	}
	changes, err := contract.SealedFor(all, ctxName)
	if err != nil {
		return "", err
	}
	if len(changes) == 0 {
		return "", errors.New(i18n.T("ask.no_sealed", ctxName))
	}
	for _, c := range changes {
		o.Change, o.Resume = c, ""
		outcome, err := deliver(ctx, out, root, cfg, o)
		if err != nil || outcome != "accepted" {
			return outcome, err
		}
	}
	return "accepted", nil
}

// humanReport prints a run as it goes, for a person at a terminal.
func humanReport(out io.Writer) func(loop.Event) {
	return func(e loop.Event) {
		switch e.Ev {
		case "node.start":
			fmt.Fprintf(out, "\n● %s\n", e.Node)
		case "edge":
			fmt.Fprintf(out, "  → %s  (%s)\n", e.To, e.Why)
		case "node.failed", "run.end", "run.cancelled":
			fmt.Fprintf(out, "  %s %s\n", e.Ev, e.Why)
		}
	}
}

// eventReport writes each event as one JSON line.
func eventReport(out io.Writer) func(loop.Event) {
	enc := json.NewEncoder(out)
	return func(e loop.Event) { _ = enc.Encode(e) }
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
// defaulting to the test section's default task and build, on a Go backend,
// to building it.
func sensorCommands(cfg *config.GofiConfig) map[string]string {
	cmds := map[string]string{}
	if cfg.Loop != nil {
		for k, v := range cfg.Loop.Sensors {
			cmds[k] = v
		}
	}
	if _, ok := cmds["build"]; !ok && backendLanguage(cfg) == config.LanguageGo {
		cmds["build"] = "go -C " + cfg.Backend.Path + " build ./..."
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
	work      string // where the session runs: the run's worktree, or root
	open      func(root string) engine.Session
	out       io.Writer
	quiet     bool // events mode: the transcript holds the tool calls
}

func (a *engineAgent) Run(ctx context.Context, t loop.Turn) (loop.TurnResult, error) {
	s := a.open(a.work)
	defer s.Close()
	path := loop.Transcript(a.root, a.run, loop.AgentID(t.Node, t.Iter))
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	transcript, err := os.Create(path)
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
					if !a.quiet {
						fmt.Fprintf(a.out, "  · %s\n", b.Name)
					}
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

// defaultGateTimeout is how long a run waits at its gate when neither the
// flag nor loop.gate_timeout says: long enough to answer from another
// terminal, short enough that a forgotten run ends in impasse, not forever.
const defaultGateTimeout = 30 * time.Minute

// gateTimeout is the wait of the gate: the flag, else loop.gate_timeout,
// else the default. 0 does not wait — a script nobody answers.
func gateTimeout(cfg *config.GofiConfig, flag string) (time.Duration, error) {
	v := flag
	if v == "" && cfg.Loop != nil {
		v = cfg.Loop.GateTimeout
	}
	if v == "" {
		return defaultGateTimeout, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("gate timeout %q: %s", v, i18n.T("run.gate_timeout_invalid"))
	}
	return d, nil
}

// askAtTerminal asks the gate's question at the supervisor's terminal, while
// it also waits for gofi answer from any other: whichever answers first.
func askAtTerminal(out io.Writer, run string) func(context.Context, *intake.Elicitation) (map[string]string, bool) {
	return func(ctx context.Context, e *intake.Elicitation) (map[string]string, bool) {
		fmt.Fprintln(out, "\n  "+i18n.T("run.gate_open", run))
		answers, err := askQuestions(ctx, e.AsQuestions())
		return answers, err == nil
	}
}

// spawnDelivery starts the delivery of a context as its own gofi process —
// gofi run --context <ctx> --flow <flow> --events — and streams its events:
// how the chat hands what a plan builds to the loop without running it inside
// its own screen.
func spawnDelivery(root string) func(flow, ctxName string) (<-chan loop.Event, func() error) {
	return func(flow, ctxName string) (<-chan loop.Event, func() error) {
		exe, err := os.Executable()
		if err != nil {
			return closedEvents(), func() error { return err }
		}
		cmd := exec.Command(exe, "run", "--context", ctxName, "--flow", flow, "--events")
		cmd.Dir = root
		var stderr strings.Builder
		cmd.Stderr = &stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return closedEvents(), func() error { return err }
		}
		if err := cmd.Start(); err != nil {
			return closedEvents(), func() error { return err }
		}
		return loop.Stream(stdout), func() error {
			if err := cmd.Wait(); err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
			}
			return nil
		}
	}
}

func closedEvents() <-chan loop.Event {
	ch := make(chan loop.Event)
	close(ch)
	return ch
}
