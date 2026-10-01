package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/doctor"
	"github.com/gofi-labs/gofi/cli/internal/githooks"
	"github.com/gofi-labs/gofi/cli/internal/graph/workspace"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"github.com/gofi-labs/gofi/cli/internal/scaffold"
	"github.com/gofi-labs/gofi/cli/internal/toolchain"
	"github.com/gofi-labs/gofi/cli/internal/tui/styles"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: i18n.T("cmd.doctor.short"),
		Long: `Run a series of environment checks and print a status table.

Checks: git on PATH, target language toolchain (go or cargo), claude CLI on PATH,
docker on PATH, write access to ~/.cache/gofi/, GitHub API connectivity, and —
when sources.institutional is set — whether the committed institutional snapshot
is behind the org repo. Each row reports ok, warning or error with a hint.`,
		Example: `gofi doctor
gofi doctor --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	}
}

func runDoctor() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		// .gofi.yaml is optional for doctor; just skip toolchain check.
		cfg = nil
	}
	checks := doctor.Run(cfg, doctor.Options{})

	// Institutional freshness — only when the project pins an org repo. The
	// doctor package can't reach sources/installed.yaml (import cycle), so the
	// check is orchestrated here and appended to the report.
	if cfg != nil && cfg.Sources.Institutional != "" {
		checks = append(checks, checkInstitutionalFreshness(cfg))
	}
	if graphEnabled(cfg) {
		root := projectRootFromCfg(cfg)
		h := projectHost()
		if h.ID == host.ClaudeCode.ID {
			checks = append(checks, checkClaudeCode())
		}
		checks = append(checks, checkInstructions(root, h), checkTiers(cfg, root, h), checkGraph(cfg, root), checkIndexLayout(root), checkMCP(root))
		if h.Guard {
			checks = append(checks, checkGuard(cfg, root))
		}
		checks = append(checks, checkHooks(cfg, root))
	}

	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
	render(checks, useColor)

	if anyFailed(checks) {
		return errors.New("one or more checks failed")
	}
	return nil
}

func render(checks []doctor.Check, color bool) {
	okStyle := lipgloss.NewStyle().Foreground(styles.Good).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Amber).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(styles.Bad).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Dim)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)

	width := 0
	for _, c := range checks {
		if l := len(c.Name); l > width {
			width = l
		}
	}

	header := "Doctor"
	if color {
		header = headerStyle.Render(header)
	}
	fmt.Println()
	fmt.Println("  " + header)

	for _, c := range checks {
		marker := "?"
		var styled string
		switch c.Status {
		case doctor.StatusOK:
			marker = "✓"
			if color {
				styled = okStyle.Render(marker)
			} else {
				styled = marker
			}
		case doctor.StatusWarn:
			marker = "!"
			if color {
				styled = warnStyle.Render(marker)
			} else {
				styled = marker
			}
		case doctor.StatusFail:
			marker = "✗"
			if color {
				styled = failStyle.Render(marker)
			} else {
				styled = marker
			}
		}

		name := padRight(c.Name, width+2)
		detail := c.Detail
		if color {
			detail = mutedStyle.Render(detail)
		}
		fmt.Printf("    %s %s%s\n", styled, name, detail)
		if c.Hint != "" && c.Status != doctor.StatusOK {
			indent := strings.Repeat(" ", 6+width+2)
			line := "→ " + c.Hint
			if color {
				line = mutedStyle.Render(line)
			}
			fmt.Println(indent + line)
		}
	}
	fmt.Println()
}

// checkInstitutionalFreshness resolves the configured institutional repo and
// compares its current SHA with the snapshot committed to this project
// (.gofi/installed.yaml). Network resolution failures degrade to a warning —
// doctor should never hard-fail over an institutional gap.
func checkInstitutionalFreshness(cfg *config.GofiConfig) doctor.Check {
	root := projectRootFromCfg(cfg)
	committed := readInstalledInstitutionalSha(root)
	resolved, err := resolveRefSHA(root, cfg.Sources.Institutional)
	return institutionalFreshnessCheck(cfg.Sources.Institutional, committed, resolved, err)
}

// institutionalFreshnessCheck is the pure decision the doctor row is built from,
// factored out so it can be unit-tested without the network.
func institutionalFreshnessCheck(ref, committed, resolved string, resolveErr error) doctor.Check {
	const name = "institutional base"
	switch {
	case resolveErr != nil:
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "could not resolve " + ref,
			Hint:   "check connectivity; then run `gofi update institutional`",
		}
	case resolved == "local":
		return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "local source (fixture)"}
	case committed == "":
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "configured but no snapshot recorded",
			Hint:   "run `gofi update institutional` to pull the org base",
		}
	case committed == resolved:
		return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "up to date (" + short(resolved) + ")"}
	default:
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: fmt.Sprintf("behind: %s → %s", short(committed), short(resolved)),
			Hint:   "run `gofi update institutional` to sync the org base",
		}
	}
}

// checkGraph reports whether the agents have a map of this code to read, and
// whether it still matches the code. A stale graph is worse than a missing one:
// it is confidently wrong.
func checkGraph(cfg *config.GofiConfig, root string) doctor.Check {
	const name = "code graph"
	ix, err := workspace.LoadIndex(root)
	if err != nil {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "not built",
			Hint:   "run `gofi index code`",
		}
	}
	var nodes int
	scopes := make([]string, 0, len(ix.Scopes))
	for _, s := range ix.Scopes {
		scopes = append(scopes, s.Name)
		nodes += s.Nodes
	}
	detail := fmt.Sprintf("%d nodes across %s", nodes, strings.Join(scopes, ", "))

	if st, err := indexStatus(cfg, root); err == nil && st.Code.State == workspace.Stale {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: detail + ", behind the code",
			Hint:   "run `gofi index code`",
		}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: detail}
}

// checkHooks reports the git hooks against what the project asked for. A
// block older releases installed calls commands that no longer exist, so it
// fails on every commit — silently — and the graph stops following the code.
func checkHooks(cfg *config.GofiConfig, root string) doctor.Check {
	const name = "git hooks"
	if legacy := githooks.Legacy(root); len(legacy) > 0 {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "obsolete gofi block in " + strings.Join(legacy, ", "),
			Hint:   "run `gofi install hooks` — it replaces the block",
		}
	}
	installed := githooks.Installed(root)
	if wantsIndexHooks(cfg) && len(installed) < len(githooks.Managed) {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "graph.hooks is on, but not every hook is installed",
			Hint:   "run `gofi install hooks`",
		}
	}
	if len(installed) == 0 {
		return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "none (graph.hooks off)"}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: strings.Join(installed, ", ")}
}

// checkMCP reports whether the project hands the index to its agents as MCP
// tools. Without the registration they still reach it through the shell, one
// process per question and one permission prompt per command.
func checkMCP(root string) doctor.Check {
	const name = "mcp server"
	h := projectHost()
	if !h.Registered(root) {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "gofi not registered in " + h.MCP.File,
			Hint:   "run `gofi install mcp`",
		}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "registered in " + h.MCP.File}
}

// checkClaudeCode fails below the minimum Claude Code: the agents would start
// without the project's AGENTS.md.
func checkClaudeCode() doctor.Check {
	const name = "claude code"
	c := toolchain.ClaudeCode(os.Getenv(EnvEngine))
	if !c.OK {
		return doctor.Check{Name: name, Status: doctor.StatusFail, Detail: c.Hint, Hint: "minimum " + toolchain.MinClaudeCode}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: c.Version + " (minimum " + toolchain.MinClaudeCode + ")"}
}

// zedFirst are instruction files the Zed editor reads in preference to
// AGENTS.md, stopping at the first it finds: a leftover from another tool
// silently replaces the project's instructions there.
var zedFirst = []string{".rules", ".cursorrules", ".windsurfrules", ".clinerules", ".github/copilot-instructions.md", "AGENT.md"}

// checkInstructions reports whether every agent will read the project's
// AGENTS.md: it has to exist, and no CLAUDE.md may be in the project, or
// Claude Code reads that instead and never AGENTS.md.
func checkInstructions(root string, h host.Host) doctor.Check {
	const name = "agent instructions"
	present := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil
	}
	if h.ID == host.ClaudeCode.ID && present(scaffold.LegacyInstructions) {
		return doctor.Check{Name: name, Status: doctor.StatusWarn,
			Detail: scaffold.LegacyInstructions + " hides " + scaffold.AgentsFile + " from Claude Code",
			Hint:   "run `gofi update agents`"}
	}
	if !present(scaffold.AgentsFile) {
		return doctor.Check{Name: name, Status: doctor.StatusWarn,
			Detail: scaffold.AgentsFile + " is missing — the agents start without the project's instructions",
			Hint:   "run `gofi update agents`"}
	}
	// Only Claude Code skips AGENTS.md for a CLAUDE.md; other hosts read both.
	for _, f := range scaffold.BlockingInstructions {
		if h.ID == host.ClaudeCode.ID && present(f) {
			return doctor.Check{Name: name, Status: doctor.StatusWarn,
				Detail: f + " hides " + scaffold.AgentsFile + " from Claude Code",
				Hint:   "move what it says into " + scaffold.AgentsFile + " and delete it"}
		}
	}
	for _, f := range zedFirst {
		if present(f) {
			return doctor.Check{Name: name, Status: doctor.StatusWarn,
				Detail: "Zed reads " + f + " instead of " + scaffold.AgentsFile,
				Hint:   "fold it into " + scaffold.AgentsFile + " and delete it, if the team uses Zed"}
		}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: scaffold.AgentsFile}
}

// checkGuard reports whether the hook that keeps agents asking the index is in
// place for the mode the project chose.
func checkGuard(cfg *config.GofiConfig, root string) doctor.Check {
	const name = "guard"
	mode := cfg.AI.GuardMode()
	if mode == config.GuardOff {
		return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "off"}
	}
	if s, err := loadClaudeSettings(root); err != nil || !s.hasGuard() {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: mode + ", but the hook is missing from " + claudeSettingsFile,
			Hint:   "run `gofi guard " + mode + "`",
		}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: mode}
}

// checkIndexLayout reports an index still kept where older releases put it.
// Nothing reads it there any more, so the agents search as if it did not exist.
func checkIndexLayout(root string) doctor.Check {
	const name = "index layout"
	if layout.HasLegacy(root) {
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "old index under .gofi/graph or .gofi/docs",
			Hint:   "run `gofi index` — it moves it to " + layout.IndexDir,
		}
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: layout.IndexDir}
}

func anyFailed(checks []doctor.Check) bool {
	for _, c := range checks {
		if c.Status == doctor.StatusFail {
			return true
		}
	}
	return false
}
