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
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/graph"
	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/pack"
	"github.com/joaoprofile/gofi/cli/internal/role"
	"github.com/joaoprofile/gofi/cli/internal/sensor"
	"github.com/joaoprofile/gofi/cli/internal/trace"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
	"github.com/joaoprofile/gofi/cli/internal/worktree"
)

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

Without a worktree the run works in the project's tree, checkpointed in
git: it starts only from a clean tree (the change's folder aside), an
accepted change becomes one commit on the current branch, and a change not
accepted leaves no commit — its work is kept on the branch gofi/<change>
and the tree goes back to where the run started.

--worktree — always the case with --detach — runs it in a git worktree of its
own, on the branch gofi/<change>, so several changes run at the same time.
The change must be committed. Accepted, the branch is committed, rebased on
the current branch once every change it depends on has landed, and
fast-forwarded into it — only while the project's tree is clean, waiting up
to --gate-timeout otherwise; never merged, never pushed. A conflict ends in
"conflito", the branch kept for the person.
The sensors run the commands of loop.sensors in .gofi.yaml; test defaults to
the test section's default task, and a stage with no command is recorded as
not checked, never as passed.

The policy is loop in .gofi.yaml: budget_usd stops the agents once spent,
max_iterations bounds the ways back to work, accept is clean or ressalvas,
and escalate (on_no_progress, on_first_reject, never) says when a role that
does not progress runs one tier up. When the run cannot go on — the contract
is wrong, or nothing progresses on the highest tier — gofi-triage proposes
the amendment in .gofi/proposals/<context>-<n>.md and the run ends in
impasse: the person decides.`,
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
	cmd.Flags().Bool("explain", false, i18n.T("cmd.run.flag.explain"))
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
	// regenerate writes a worktree's derived files; in a test, the running
	// binary is the test binary and must not be run as gofi.
	regenerate func(ctx context.Context, dir string) error
	// graph and trace are gofi's own sensors' commands, for the same reason.
	graph, trace *pack.Command
}{}

// graphSensor is gofi's own stage (0002, D11, S4): this gofi indexes the
// code the turn changed, in the tree the run works on, and holds it to the
// graph the run started from — the architecture rules the change broke, in
// SARIF, and its diff, for the run to dispatch and scope the audit by.
func graphSensor(run string) pack.Command {
	if runPorts.graph != nil {
		return *runPorts.graph
	}
	exe, err := os.Executable()
	if err != nil {
		return pack.Command{}
	}
	baseline := path.Join(loop.BaselineDir(run), graph.GraphFile)
	return pack.Command{
		Run:    fmt.Sprintf("%s index code --baseline %s --diff %s --report %s", strconv.Quote(exe), baseline, loop.DiffFile(run), loop.GraphReport(run)),
		Format: pack.FormatSARIF, Report: loop.GraphReport(run),
	}
}

// traceSensor is gofi's own stage too (0002, D10, S5): every requirement
// the change adds or modifies has the code that implements it, every
// scenario its test — a gap is a finding at the scenario, in SARIF.
func traceSensor(change, run string) pack.Command {
	if runPorts.trace != nil {
		return *runPorts.trace
	}
	exe, err := os.Executable()
	if err != nil {
		return pack.Command{}
	}
	report := path.Join(loop.RunsDir, run, "trace.sarif")
	return pack.Command{Run: fmt.Sprintf("%s trace %s --report %s", strconv.Quote(exe), change, report), Format: pack.FormatSARIF, Report: report}
}

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
	if explain, _ := cmd.Flags().GetBool("explain"); explain {
		if len(args) == 0 {
			return errors.New(i18n.T("run.explain_which"))
		}
		flowName, _ := cmd.Flags().GetString("flow")
		return explainRun(cmd.OutOrStdout(), root, cfg, args[0], flowName)
	}
	// A change not approved yet goes through the contract first, here, with
	// the person — even when the delivery itself runs detached.
	if len(args) > 0 {
		events, _ := cmd.Flags().GetBool("events")
		if err := contractGate(cmd, root, cfg, args[0], !events); err != nil {
			return err
		}
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
		if s.Node == "" && !landable(s) {
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
	policy := loopPolicy(cfg)
	if err := policy.Check(); err != nil {
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
	// In the project's own tree, the run is checkpointed in git (0002, D14):
	// it starts from a clean tree, and its end is one commit, or its work
	// kept on a branch of its own. A tree it cannot start on is said before
	// anything of the run is written.
	var cp *worktree.Checkpoint
	if inPlace := prior != nil && prior.Worktree == "" || prior == nil && !o.Worktree; inPlace {
		if cp, err = checkpoint(ctx, root, run, change, prior); err != nil {
			return "", err
		}
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
		tree.Derived, tree.Regenerate = derivedFile, regenerateIndexes
		if runPorts.regenerate != nil {
			tree.Regenerate = runPorts.regenerate
		}
	}
	// What the run starts — the agents, the gofi MCP server, the git hooks
	// of its commit — works on the worktree, not on the project it was
	// copied from.
	if tree != nil {
		_ = os.Setenv(config.EnvRoot, work)
	} else {
		_ = os.Unsetenv(config.EnvRoot)
	}
	report := humanReport(out)
	if o.Events {
		report = eventReport(out)
	}
	// Accepted and archived, a run in a worktree that has not landed yet
	// only lands: resuming it runs no agent again.
	if prior != nil && tree != nil && landable(prior) {
		return landRun(ctx, out, root, prior, tree, wait, o.Events, report)
	}

	open, err := turnSessions(cfg, work, o.Mode)
	if err != nil {
		return "", err
	}
	roles, problems := role.Load(layout.RoleDirs(work)...)
	if len(problems) > 0 {
		return "", errors.Join(problems...)
	}
	facts, err := changeFacts(cfg, work, change)
	if err != nil {
		return "", err
	}
	sensors, err := sensorCommands(cfg)
	if err != nil {
		return "", err
	}
	// The language's pack also says which files are tests: what the loop
	// sends to the test role, not to the code's.
	langPack, err := pack.Load(backendLanguage(cfg))
	if err != nil {
		return "", err
	}
	if _, set := sensors["graph"]; !set {
		sensors["graph"] = graphSensor(run)
	}
	if _, set := sensors["trace"]; !set {
		sensors["trace"] = traceSensor(change, run)
	}
	if err := captureBaseline(work, run); err != nil {
		return "", fmt.Errorf("baseline do grafo: %w", err)
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
		Root: root, Work: work, Flow: f, Brief: loop.DefaultBrief, Roles: roles, Facts: facts,
		Agent:    &engineAgent{root: root, work: work, run: run, open: open, out: out, quiet: o.Events},
		Sensors:  sensor.Runner{Root: work, Commands: sensors},
		Verdicts: diskVerdicts{root: work},
		Gate:     gate,
		Policy:   policy,
		IsTest:   langPack.IsTest,
		IsSource: langPack.IsSource,
		Reach:    planReach(work, run, backendLanguage(cfg)),
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
		} else if cp != nil {
			s, err = r.StartIn(ctx, run, change, ctxName, func(s *loop.State) { s.Base = cp.Branch })
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
	// The report is written last, with how the run ended — committed, kept,
	// landed —; an accepted change's trace is read before it is archived.
	var rows []trace.Row
	if s.Outcome == "accepted" {
		rows, _ = traceRows(work, cfg, change)
		if rows == nil {
			rows = []trace.Row{}
		}
	}
	defer func() {
		if p, err := writeReport(root, s, rows); err == nil && !o.Events {
			rel, _ := filepath.Rel(root, p)
			fmt.Fprintln(out, "  "+i18n.T("run.report", filepath.ToSlash(rel), s.Run))
		}
	}()
	if s.Outcome != "accepted" && cp != nil {
		return s.Outcome, keepWork(ctx, out, root, s, cp, o.Events, report)
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
	if cp != nil {
		message := commitMessage(work, s)
		if err := cp.Commit(ctx, message, "specs/changes/"+change, filepath.ToSlash(res.Dir)); err != nil {
			return s.Outcome, fmt.Errorf("commit %s: %w", cp.Branch, err)
		}
		s.Landing = landed
		_ = loop.SaveState(root, s)
		report(loop.AppendEvent(root, s.Run, loop.Event{Ev: "land.done", Node: s.Run, Why: cp.Branch}))
		if !o.Events {
			subject, _, _ := strings.Cut(message, "\n")
			fmt.Fprintln(out, "  "+i18n.T("run.checkpoint.done", subject, cp.Branch))
		}
	}
	if tree == nil {
		return s.Outcome, nil
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

// explainRun shows, without running anything, what a change's run would
// dispatch: its facts, and at each agent node of the flow the roles of its
// stage, whether each runs and why (0002, D18). A node past the sensors is
// read as the sensors left it green — the only way the flow reaches it.
func explainRun(out io.Writer, root string, cfg *config.GofiConfig, change, flowName string) error {
	if _, err := sealedChange(root, change); err != nil {
		return err
	}
	f, err := loop.Load(root, flowName)
	if err != nil {
		return err
	}
	roles, problems := role.Load(layout.RoleDirs(root)...)
	if len(problems) > 0 {
		return errors.Join(problems...)
	}
	facts, err := changeFacts(cfg, root, change)
	if err != nil {
		return err
	}
	var held []string
	for k := range facts {
		held = append(held, k)
	}
	slices.Sort(held)
	fmt.Fprintf(out, "  %s %s\n", i18n.T("run.explain.facts", change), strings.Join(held, ", "))
	// The layers with work are the plan's, known once the run planned: with
	// a plan stage that dispatches, every layer may; with none, plan.none.
	planned := false
	for _, n := range f.Nodes {
		if n.Kind == loop.KindAgent && n.Stage == "plan" && len(role.Dispatch(roles, "plan", facts)) > 0 {
			planned = true
		}
	}
	for _, name := range f.Walk() {
		n := f.Nodes[name]
		if n.Kind != loop.KindAgent {
			continue
		}
		if n.Role != "" {
			fmt.Fprintf(out, "\n  %s — %s\n", name, i18n.T("run.explain.fixed", n.Role))
			continue
		}
		at := role.Facts{}
		for k, v := range facts {
			at[k] = v
		}
		at["sensor.green"] = true
		at["tests.due"] = true
		if planned {
			for _, l := range role.Layers {
				at["layer("+l+")"] = true
			}
		} else {
			at["plan.none"] = true
		}
		fmt.Fprintf(out, "\n  %s — %s %s\n", name, i18n.T("run.explain.stage"), n.Stage)
		if planned && n.Stage == "implement" {
			fmt.Fprintln(out, "    "+i18n.T("run.explain.layers"))
		}
		for _, line := range role.Explain(roles, n.Stage, at) {
			fmt.Fprintln(out, "    "+line)
		}
	}
	return nil
}

// changeFacts are what holds about a change before its run starts: the
// surfaces it touches, as its proposal declares them — or, declaring none,
// the project's backend, else its one other surface.
func changeFacts(cfg *config.GofiConfig, root, change string) (role.Facts, error) {
	all, err := contract.Docs(root)
	if err != nil {
		return nil, err
	}
	var surfaces []string
	for _, c := range contract.Changes(all) {
		if c.ID == change {
			surfaces = c.Surfaces
		}
	}
	if len(surfaces) == 0 {
		if project := projectSurfaces(cfg); len(project) > 0 {
			surfaces = project[:1]
		}
	}
	facts := role.Facts{}
	for _, s := range surfaces {
		facts["change.surface("+s+")"] = true
	}
	return facts, nil
}

// projectSurfaces are the surfaces a project has, backend first: what a
// change can touch.
func projectSurfaces(cfg *config.GofiConfig) []string {
	var out []string
	if cfg.Backend != nil {
		out = append(out, "backend")
	}
	for _, s := range cfg.UISurfaces() {
		if s.Name == "mobile" {
			out = append(out, "mobile")
		} else {
			out = append(out, "web")
		}
	}
	if cfg.Ops != nil {
		out = append(out, "infra")
	}
	return out
}

// derivedFile is a file gofi generates from the rest of the tree: the index
// and graph under .gofi/index/, and the INDEX.md of each document folder.
// Two runs side by side both rewrite them; on landing they are generated
// again, never a conflict.
func derivedFile(path string) bool {
	return strings.HasPrefix(path, ".gofi/index/") || filepath.Base(path) == "INDEX.md"
}

// regenerateIndexes writes the derived files of a worktree again, with the
// gofi that supervises the run.
func regenerateIndexes(ctx context.Context, dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	for _, target := range []string{"code", "docs"} {
		cmd := exec.CommandContext(ctx, exe, "index", target)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), config.EnvRoot+"="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gofi index %s: %v\n%s", target, err, out)
		}
	}
	return nil
}

// landable is a run accepted in a worktree that has not landed: resumed, it
// only lands.
func landable(s *loop.State) bool {
	return s.Node == "" && s.Outcome == "accepted" && s.Worktree != "" && s.Landing != landed
}

// landPoll is how often a waiting landing looks again.
var landPoll = 10 * time.Second

// landRun lands an accepted run's branch on the base: once every change it
// depends on has landed, and while the project's tree is clean — waiting up
// to wait otherwise, the person's work never touched. Landed, the worktree
// goes; a conflict keeps the branch for the person. The outcome is accepted
// only when it landed.
func landRun(ctx context.Context, out io.Writer, root string, s *loop.State, tree *worktree.Tree, wait time.Duration, events bool, report func(loop.Event)) (string, error) {
	say := func(ev, why string) {
		report(loop.AppendEvent(root, s.Run, loop.Event{Ev: ev, Node: s.Run, Why: why}))
	}
	// Committed here, not where the run was accepted: a resume that lands a
	// run whose commit failed commits it first. Nothing left to commit is
	// not an error.
	message := commitMessage(tree.Dir, s)
	if tree.Ahead(ctx) {
		message = "chore: regenerate the indexes of " + s.Change
	}
	if err := tree.Commit(ctx, message); err != nil {
		return waiting, fmt.Errorf("commit %s: %w", tree.Branch, err)
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
			return conflict, fmt.Errorf("%s: %w", i18n.T("run.land.conflict", tree.Branch, tree.Base, tree.Dir), err)
		} else if errors.Is(err, worktree.ErrDirty) || errors.Is(err, worktree.ErrMoved) || errors.Is(err, worktree.ErrRaced) {
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

// commitMessage is the commit an accepted change becomes.
func commitMessage(work string, s *loop.State) string {
	fallback := fmt.Sprintf("feat(%s): %s", s.Context, s.Change)
	data, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(loop.CommitFile(s.Run))))
	if err != nil {
		return fallback
	}
	// The delivery's message, when it reads as one: a subject line of a
	// commit, then the body.
	msg := strings.TrimSpace(string(data))
	msg = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(msg, "```"), "```"))
	subject, _, _ := strings.Cut(msg, "\n")
	if subject == "" || len(subject) > 100 || strings.HasPrefix(subject, "#") {
		return fallback
	}
	return msg
}

// checkpoint is the git checkpoint of a run in the project's own tree; nil
// when the project is not in a repository with a commit. A new run starts
// only from a clean tree — the open changes, approved and maybe not yet
// committed, and the derived files aside —, a resumed one only on the
// branch it started on.
func checkpoint(ctx context.Context, root, run, change string, prior *loop.State) (*worktree.Checkpoint, error) {
	if prior != nil && prior.Base == "" {
		return nil, nil // started with no checkpoint: it ends with none
	}
	cp, ok := worktree.OpenCheckpoint(ctx, root)
	if !ok {
		return nil, nil
	}
	cp.Keep = []string{"specs/changes"}
	cp.Derived, cp.Regenerate = derivedFile, regenerateIndexes
	if runPorts.regenerate != nil {
		cp.Regenerate = runPorts.regenerate
	}
	if prior != nil {
		if prior.Base != cp.Branch {
			return nil, errors.New(i18n.T("run.checkpoint.moved", run, prior.Base, cp.Branch, prior.Base))
		}
		return cp, nil
	}
	dirty, err := cp.Dirty(ctx)
	if err != nil {
		return nil, err
	}
	if len(dirty) > 0 {
		if len(dirty) > 5 {
			dirty = append(dirty[:5], "…")
		}
		return nil, errors.New(i18n.T("run.dirty", change, strings.Join(dirty, ", ")))
	}
	return cp, nil
}

// keepWork leaves no commit of a run not accepted: its work goes to a
// branch of its own, gofi/<change>, and the tree back to where the run
// started (0002, D14).
func keepWork(ctx context.Context, out io.Writer, root string, s *loop.State, cp *worktree.Checkpoint, events bool, report func(loop.Event)) error {
	branch, err := cp.Shelve(ctx, "gofi/"+s.Change, fmt.Sprintf("wip(%s): %s — %s (%s)", s.Context, s.Change, s.Outcome, s.Run))
	if err != nil {
		return fmt.Errorf("keep the work of %s: %w", s.Run, err)
	}
	if branch == "" {
		return nil
	}
	s.Kept = branch
	_ = loop.SaveState(root, s)
	report(loop.AppendEvent(root, s.Run, loop.Event{Ev: "work.kept", Node: s.Run, To: branch}))
	if !events {
		fmt.Fprintln(out, "  "+i18n.T("run.checkpoint.kept", branch))
	}
	return nil
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
		case "node.failed", "run.end", "run.cancelled", "land.conflict":
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

// sensorCommands are the commands of each stage: loop.sensors first, then
// the backend language's pack (0002, D13); with no pack, test is the test
// section's default task. A command from loop.sensors is read as
// file:line output, the shape compilers and most tools print.
func sensorCommands(cfg *config.GofiConfig) (map[string]pack.Command, error) {
	cmds := map[string]pack.Command{}
	if lang := backendLanguage(cfg); lang != "" {
		p, err := pack.Load(lang)
		if err != nil {
			return nil, err
		}
		if p != nil {
			for stage, c := range p.Sensors {
				cmds[stage] = c.Expand(cfg.Backend.Path)
			}
		}
	}
	if _, ok := cmds["test"]; !ok {
		if t, ok := cfg.Test.Tasks[cfg.Test.Default]; ok && !strings.HasPrefix(strings.TrimSpace(t.Run), "echo ") {
			cmds["test"] = pack.Command{Run: t.Run, Format: pack.FormatLines}
		}
	}
	if cfg.Loop != nil {
		for stage, run := range cfg.Loop.Sensors {
			cmds[stage] = pack.Command{Run: run, Format: pack.FormatLines}
		}
	}
	return cmds, nil
}

// turnSessions opens the session of a role's turn, in work: on the
// project's host (ai.host), on its tier's model — an internal role has no
// skill frontmatter to name one —, with the tools its contract grants.
func turnSessions(cfg *config.GofiConfig, work, mode string) (func(dir string, tier host.Tier, grant []string) engine.Session, error) {
	if runPorts.open != nil {
		return func(dir string, _ host.Tier, _ []string) engine.Session { return runPorts.open(dir) }, nil
	}
	if mode == "" {
		mode = "acceptEdits"
	}
	session, err := agentSessions(cfg, work, mode, append(append([]string{}, guard.ReadOnlyRules...), guard.WriteRules...))
	if err != nil {
		return nil, err
	}
	h, _ := host.Get(cfg.AI.Host)
	return func(_ string, t host.Tier, grant []string) engine.Session {
		model := ""
		if h.SkillModel && t != "" {
			model = h.Model(t, cfg.AI.Tiers)
		}
		return session(model, grant)
	}, nil
}

// engineAgent runs a role's turn in a fresh engine session and keeps what
// the agent did in agents/<node>-<iter>.ndjson.
type engineAgent struct {
	root, run string
	work      string // where the session runs: the run's worktree, or root
	open      func(dir string, tier host.Tier, grant []string) engine.Session
	out       io.Writer
	quiet     bool // events mode: the transcript holds the tool calls
	// transcript, when set, is where the turn is recorded instead of the
	// run's agents folder: a turn outside a run.
	transcript string
}

func (a *engineAgent) Run(ctx context.Context, t loop.Turn) (loop.TurnResult, error) {
	s := a.open(a.work, t.Tier, t.Tools)
	defer s.Close()
	id := t.Agent
	if id == "" {
		id = loop.AgentID(t.Node, t.Iter)
	}
	path := a.transcript
	if path == "" {
		path = loop.Transcript(a.root, a.run, id)
	}
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

type diskVerdicts struct{ root string }

func (d diskVerdicts) Read(ctx string) (*verdict.Verdict, error) { return verdict.Read(d.root, ctx) }

func (d diskVerdicts) ReadLens(ctx, lens string) (*verdict.Verdict, error) {
	return verdict.ReadLens(d.root, ctx, lens)
}

// Write records the verdict merged from the lenses as gofi verdict write
// does: checked, and carried into the context's memory.
func (d diskVerdicts) Write(v *verdict.Verdict) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = writeVerdict(d.root, v.Context, data, v.Run, v.Iter, "")
	return err
}

// loopPolicy is the project's convergence policy, from loop in .gofi.yaml:
// absent, the defaults — no budget, the flow's limits, clean, on_no_progress.
func loopPolicy(cfg *config.GofiConfig) loop.Policy {
	if cfg.Loop == nil {
		return loop.Policy{}
	}
	l := cfg.Loop
	return loop.Policy{MaxIterations: l.MaxIterations, BudgetUSD: l.BudgetUSD, Accept: l.Accept, Escalate: l.Escalate, AuditParallel: l.AuditParallel}
}

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
