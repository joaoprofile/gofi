package cli

// The context benchmark: how much context a gofi project costs an agent, how
// well its search answers, and whether the agent searches before it reads.
//
// It is the yardstick of the context-engineering work, run before a change and
// after it, so it is reproducible by construction: the fixture project is
// installed from this repository by the same code `gofi init` runs, and every
// question and task lives in bench/ under version control.
//
//	GOFI_BENCH=1 go test ./internal/cli -run Bench -v            # sizes and search: free
//	GOFI_BENCH=1 GOFI_BENCH_LIVE=1 go test ./internal/cli -run Bench -v -timeout 30m
//	GOFI_BENCH_LIVE=tokens | task runs one half of the live measures.
//
// GOFI_BENCH_LABEL names the report (bench/results/<label>.md); GOFI_BENCH_MODEL
// and GOFI_BENCH_TASK_MODELS (comma-separated) pick the models of the live runs.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/docs"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/retrieval"
	"github.com/gofi-labs/gofi/cli/internal/scaffold"
	"github.com/gofi-labs/gofi/cli/internal/tui/wizard"
)

// benchReport is what one run measured. Its JSON sits next to the markdown so
// two runs can be compared by a tool, not only by eye.
type benchReport struct {
	Label  string    `json:"label"`
	When   time.Time `json:"when"`
	Commit string    `json:"commit"`
	Model  string    `json:"model,omitempty"`
	Sizes  []sizeRow `json:"sizes"`
	Search searchRow `json:"search"`
	// Holdout is the held-out set, scored only when asked for: tuning against
	// it would make it a second development set.
	Holdout *searchRow `json:"holdout,omitempty"`
	Live    []liveRow  `json:"live,omitempty"`
	Tasks   []taskRow  `json:"tasks,omitempty"`
	Skipped []string   `json:"skipped,omitempty"`
}

type sizeRow struct {
	What      string `json:"what"`
	Bytes     int    `json:"bytes"`
	Loaded    string `json:"loaded"`
	EstTokens int    `json:"est_tokens"`
}

type searchRow struct {
	Engine    string   `json:"engine"`
	Questions int      `json:"questions"`
	Recall1   int      `json:"recall1"`
	Recall3   int      `json:"recall3"`
	Recall5   int      `json:"recall5"`
	Section3  int      `json:"section_top3,omitempty"`
	MRR       float64  `json:"mrr"`
	AvgTokens int      `json:"avg_tokens_to_answer"`
	Misses    []string `json:"misses"`
}

type liveRow struct {
	Scenario string  `json:"scenario"`
	Context  int     `json:"context_tokens"`
	Delta    int     `json:"delta_vs_baseline"`
	CostUSD  float64 `json:"cost_usd"`
}

type taskRow struct {
	Model         string   `json:"model"`
	Guard         string   `json:"guard"`
	Correct       bool     `json:"correct"`
	Tools         []string `json:"tools"`
	FirstSearch   string   `json:"first_search"`
	GofiBefore    bool     `json:"gofi_before_grep_or_full_read"`
	GofiCalls     int      `json:"gofi_calls"`
	GrepGlobCalls int      `json:"grep_glob_calls"`
	FullReads     int      `json:"full_reads"`
	Context       int      `json:"context_tokens"`
	CostUSD       float64  `json:"cost_usd"`
	Answer        string   `json:"answer"`
}

func TestBench(t *testing.T) {
	if os.Getenv("GOFI_BENCH") == "" {
		t.Skip("set GOFI_BENCH=1 to run the context benchmark")
	}
	repo := benchRepoRoot(t)
	work := t.TempDir()
	gofiBin := buildGofi(t, work)
	proj := benchFixture(t, repo, filepath.Join(work, "proj"))
	runIn(t, proj, gofiBin, "index")

	rep := &benchReport{
		Label:  envOr("GOFI_BENCH_LABEL", "run-"+time.Now().Format("2006-01-02-1504")),
		When:   time.Now().UTC().Truncate(time.Second),
		Commit: gitHead(repo),
	}
	rep.Sizes = benchSizes(t, proj)
	rep.Search = benchSearch(t, repo, proj, "golden-knowledge.md", "retrieval core")
	if os.Getenv("GOFI_BENCH_HOLDOUT") != "" {
		h := benchSearch(t, repo, proj, "golden-holdout.md", "held out")
		rep.Holdout = &h
	}

	// GOFI_BENCH_LIVE: 1 or all runs everything; tokens or task runs one half.
	if live := os.Getenv("GOFI_BENCH_LIVE"); live != "" {
		if _, err := exec.LookPath("claude"); err != nil {
			rep.Skipped = append(rep.Skipped, "live: claude not on PATH")
		} else {
			if live != "task" {
				rep.Model = envOr("GOFI_BENCH_MODEL", "haiku")
				rep.Live = benchLive(t, work, proj, gofiBin, rep.Model)
			}
			if live != "tokens" {
				// Every model under the guard the project ships with, and the
				// first again with the guard off: the first comparison picks a
				// model, the second says whether the guard changes behaviour.
				models := strings.Split(envOr("GOFI_BENCH_TASK_MODELS", "haiku,sonnet,opus"), ",")
				for _, m := range models {
					rep.Tasks = append(rep.Tasks, *benchTask(t, proj, gofiBin, repo, strings.TrimSpace(m), config.GuardWarn))
				}
				rep.Tasks = append(rep.Tasks, *benchTask(t, proj, gofiBin, repo, strings.TrimSpace(models[0]), config.GuardOff))
			}
		}
	} else {
		rep.Skipped = append(rep.Skipped, "live token and task runs: set GOFI_BENCH_LIVE=1")
	}

	out := filepath.Join(repo, "bench", "results")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	js, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(filepath.Join(out, rep.Label+".json"), append(js, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	md := rep.markdown()
	if err := os.WriteFile(filepath.Join(out, rep.Label+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + md)
}

// --- fixture ------------------------------------------------------------

func benchRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	if _, err := os.Stat(filepath.Join(dir, "ai", "skills")); err != nil {
		t.Fatalf("%s is not the gofi repository: %v", dir, err)
	}
	return dir
}

func buildGofi(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin", "gofi")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gofi")
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gofi: %v\n%s", err, out)
	}
	return bin
}

// benchFixture installs a project the way gofi init does, from this
// repository instead of a download, and adds a small context — spec and code —
// for the task run to work on.
func benchFixture(t *testing.T, repo, proj string) string {
	t.Helper()
	fsys := os.DirFS(repo)
	data := scaffold.TemplateData{ProjectName: "acme", Date: "2026-01-01", Language: "go", Module: "example.com/acme", SourceRoot: "backend", Agents: nil}
	if _, err := scaffold.InstallAgentsContent(fsys, ".", proj, data, scaffold.InstallNew); err != nil {
		t.Fatalf("install agents content: %v", err)
	}
	if err := installSDKLayer(fsys, proj, "go", "", scaffold.InstallNew); err != nil {
		t.Fatalf("install sdk: %v", err)
	}
	installDSLayer(fsys, proj, []string{"web"}, scaffold.InstallNew)
	if _, err := scaffold.InstallInstitutionalSeed(fsys, ".", proj, "acme", data); err != nil {
		t.Fatalf("install institutional: %v", err)
	}
	// The configuration gofi init would write for this project.
	cfg := buildConfig(&wizard.Result{
		AIHost: config.AIHostClaudeVSCode, AIModel: config.DefaultModel,
		Name: "acme", Root: proj, Environments: []string{wizard.EnvBack},
		Language: config.LanguageGo, SourcePath: "backend", Module: "example.com/acme",
		AgentsRef: config.DefaultAgentsRef,
	})
	if err := config.Save(filepath.Join(proj, config.FileName), cfg); err != nil {
		t.Fatal(err)
	}
	// What gofi init wires for the agents: the MCP server, the guard and the
	// read-only queries allowed ahead of time.
	if _, err := host.ClaudeCode.RegisterMCP(proj); err != nil {
		t.Fatal(err)
	}
	if _, err := installAgentSettings(proj, config.AI{Guard: config.GuardWarn}); err != nil {
		t.Fatal(err)
	}
	for rel, body := range benchContext {
		p := filepath.Join(proj, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=bench", "-c", "user.email=bench@example.com", "commit", "-qm", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = proj
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return proj
}

// benchContext is one context — a spec with a business rule and the code that
// enforces it — which the task asks the agent to connect.
var benchContext = map[string]string{
	"specs/order/sdd-order.md": `---
tipo: spec
formato: 2
contexto: order
versao: "1.0"
status: aprovado
entidades: [order_order]
operacoes: [faturar, cancelar]
keywords: [pedido, faturamento]
---
# SDD — Pedido

## 3. Modelo de Dados

Tabela order_order com status (aberto, cancelado, faturado).

## 5. Regras de Negócio

### RN-01 — Pedido cancelado não pode ser faturado

Faturar um pedido com status cancelado é recusado com erro de conflito.

### RN-02 — Pedido faturado não pode ser cancelado

Cancelar um pedido já faturado é recusado.
`,
	"backend/go.mod": "module example.com/acme\n\ngo 1.22\n",
	"backend/domain/order/model/order.go": `package model

// Status is where an order is in its life.
type Status string

const (
	StatusOpen      Status = "aberto"
	StatusCancelled Status = "cancelado"
	StatusBilled    Status = "faturado"
)

// Order is a customer order.
type Order struct {
	ID     int64
	Status Status
}
`,
	"backend/domain/order/service/order_service.go": `//gofi:context order
package service

import (
	"errors"

	"example.com/acme/domain/order/model"
)

// ErrConflict is returned when an order cannot move to the requested status.
var ErrConflict = errors.New("order status conflict")

// Bill charges an order.
func Bill(o *model.Order) error {
	if o.Status == model.StatusCancelled {
		return ErrConflict
	}
	o.Status = model.StatusBilled
	return nil
}

// Cancel withdraws an order.
func Cancel(o *model.Order) error {
	if o.Status == model.StatusBilled {
		return ErrConflict
	}
	o.Status = model.StatusCancelled
	return nil
}
`,
}

func runIn(t *testing.T, dir, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFI_NO_SETUP=1", "NO_COLOR=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gofi %v: %v\n%s", args, err, out)
	}
}

// --- free measures ----------------------------------------------------------

// estTokens approximates tokens from bytes. The ratio is only for comparing
// sizes between runs; the live runs count exactly.
func estTokens(b int) int { return b * 10 / 36 }

func benchSizes(t *testing.T, proj string) []sizeRow {
	t.Helper()
	var rows []sizeRow
	add := func(what, rel, loaded string) {
		b, err := os.ReadFile(filepath.Join(proj, filepath.FromSlash(rel)))
		if err != nil {
			return
		}
		rows = append(rows, sizeRow{What: what, Bytes: len(b), Loaded: loaded, EstTokens: estTokens(len(b))})
	}
	add("AGENTS.md", "AGENTS.md", "every session")
	skills, _ := filepath.Glob(filepath.Join(proj, ".claude", "skills", "*", "SKILL.md"))
	sort.Strings(skills)
	for _, s := range skills {
		rel, _ := filepath.Rel(proj, s)
		add(filepath.Base(filepath.Dir(s)), filepath.ToSlash(rel), "on invocation")
	}
	return rows
}

// benchSearch scores one golden file. GOFI_BENCH_VERBOSE logs the first
// answers of every question, which is what ranking work reads.
func benchSearch(t *testing.T, repo, proj, file, label string) searchRow {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join(repo, "bench", file))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(proj, filepath.FromSlash(docs.GoldenFile()))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, golden, 0o644); err != nil {
		t.Fatal(err)
	}
	questions, err := docs.ReadGolden(proj)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := retrieval.Open(proj, config.LanguageGo)
	if err != nil {
		t.Fatal(err)
	}
	ev := engine.Evaluate(questions)
	if os.Getenv("GOFI_BENCH_VERBOSE") != "" && file != "golden-holdout.md" {
		for _, q := range questions {
			var b strings.Builder
			for i, h := range engine.Find(retrieval.Query{Text: q.Ask, Limit: 3}) {
				mark := " "
				if h.Path == q.Document {
					mark = "*"
					if h.Title == q.Section {
						mark = "§"
					}
				}
				fmt.Fprintf(&b, "\n    %d%s %s § %s", i+1, mark, strings.TrimPrefix(h.Path, ".claude/"), h.Title)
			}
			t.Logf("%s %s  → want §%s%s", q.ID, q.Ask, q.Section, b.String())
		}
	}
	row := searchRow{Engine: label, Questions: ev.Questions, Recall1: ev.DocTop1, Recall3: ev.DocTop3, Recall5: ev.DocTop5, Section3: ev.SectionTop3, MRR: ev.MRR}
	for _, q := range ev.Misses {
		row.Misses = append(row.Misses, q.ID+" "+q.Ask)
	}
	return row
}

// --- live measures ----------------------------------------------------------

// claudeRun is what one engine run reported.
type claudeRun struct {
	Context int
	CostUSD float64
	Result  string
	Tools   []toolCall
}

type toolCall struct {
	Name  string
	Input map[string]any
}

// runClaude sends one message and collects the usage and every tool call.
func runClaude(t *testing.T, dir, prompt string, extraEnv []string, args ...string) claudeRun {
	t.Helper()
	base := []string{"--print", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
		"--no-session-persistence", "--max-budget-usd", "1"}
	cmd := exec.Command("claude", append(base, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	msg, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user",
		"content": []map[string]string{{"type": "text", "text": prompt}}}})
	cmd.Stdin = bytes.NewReader(append(msg, '\n'))
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		t.Fatalf("claude: %v", err)
	}
	var run claudeRun
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var l struct {
			Type    string  `json:"type"`
			Result  string  `json:"result"`
			Cost    float64 `json:"total_cost_usd"`
			Message *struct {
				Content []struct {
					Type  string         `json:"type"`
					Name  string         `json:"name"`
					Input map[string]any `json:"input"`
				} `json:"content"`
				Usage *struct {
					Input      int `json:"input_tokens"`
					CacheWrite int `json:"cache_creation_input_tokens"`
					CacheRead  int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		switch l.Type {
		case "assistant":
			if l.Message == nil {
				continue
			}
			for _, c := range l.Message.Content {
				if c.Type == "tool_use" {
					run.Tools = append(run.Tools, toolCall{Name: c.Name, Input: c.Input})
				}
			}
			// The largest request of the run is its context: every later turn
			// carries all the earlier ones.
			if u := l.Message.Usage; u != nil {
				run.Context = max(run.Context, u.Input+u.CacheWrite+u.CacheRead)
			}
		case "result":
			run.Result, run.CostUSD = l.Result, l.Cost
		}
	}
	return run
}

// benchLive measures what the harness adds to a session, and what each skill
// adds when invoked. Tools are off, so a skill is loaded and answered in one
// turn instead of starting its work.
func benchLive(t *testing.T, work, proj, gofiBin, model string) []liveRow {
	t.Helper()
	const ask = "Benchmark run: reply with the single word OK and nothing else."
	bare := filepath.Join(work, "bare")
	_ = os.MkdirAll(bare, 0o755)
	flags := []string{"--model", model, "--tools", ""}
	// The project's hooks run gofi: this build, not whatever is installed.
	env := []string{"PATH=" + filepath.Dir(gofiBin) + string(os.PathListSeparator) + os.Getenv("PATH"), "GOFI_NO_SETUP=1"}

	base := runClaude(t, bare, ask, env, flags...)
	rows := []liveRow{{Scenario: "empty folder (Claude Code alone)", Context: base.Context, CostUSD: base.CostUSD}}
	session := runClaude(t, proj, ask, env, flags...)
	rows = append(rows, liveRow{Scenario: "gofi project, session start", Context: session.Context, Delta: session.Context - base.Context, CostUSD: session.CostUSD})

	skills, _ := filepath.Glob(filepath.Join(proj, ".claude", "skills", "*", "SKILL.md"))
	sort.Strings(skills)
	for _, s := range skills {
		name := filepath.Base(filepath.Dir(s))
		r := runClaude(t, proj, "/"+name+" "+ask, env, flags...)
		rows = append(rows, liveRow{Scenario: "/" + name, Context: r.Context, Delta: r.Context - session.Context, CostUSD: r.CostUSD})
	}
	return rows
}

// benchTask gives the agent a real question that spans the spec and the code,
// and records how it searched: the order of its tool calls is the measure of
// whether it goes to the graph before it greps or reads whole files.
func benchTask(t *testing.T, proj, gofiBin, repo, model, guard string) *taskRow {
	t.Helper()
	task, err := os.ReadFile(filepath.Join(repo, "bench", "task-order-rule.md"))
	if err != nil {
		t.Fatal(err)
	}
	setGuard(t, proj, guard)
	env := []string{"PATH=" + filepath.Dir(gofiBin) + string(os.PathListSeparator) + os.Getenv("PATH"), "GOFI_NO_SETUP=1", "NO_COLOR=1"}
	// The project's MCP server is trusted here the way gofi chat trusts it; a
	// headless run has nobody to answer the first-use prompt.
	settings := `{"enabledMcpjsonServers":["gofi"]}`
	r := runClaude(t, proj, string(task), env, "--model", model, "--settings", settings,
		"--allowedTools", "Bash(gofi *)", "mcp__gofi__*", "Read", "Grep", "Glob")

	answer := strings.TrimSpace(r.Result)
	row := &taskRow{Model: model, Guard: guard, Context: r.Context, CostUSD: r.CostUSD, Answer: answer, Correct: correctAnswer(answer)}
	sawNonGofi := false
	for _, c := range r.Tools {
		label, isGofi, isSearch := classify(c)
		row.Tools = append(row.Tools, label)
		if isSearch && row.FirstSearch == "" {
			row.FirstSearch = label
		}
		switch {
		case isGofi:
			row.GofiCalls++
			if !sawNonGofi {
				row.GofiBefore = true
			}
		case c.Name == "Grep" || c.Name == "Glob":
			row.GrepGlobCalls++
			sawNonGofi = true
		case c.Name == "Read" && c.Input["offset"] == nil && c.Input["limit"] == nil:
			row.FullReads++
			sawNonGofi = true
		}
	}
	return row
}

// correctAnswer grades the task: the rule is in the spec's RN-01 and in
// order_service.go's Bill. An answer naming both places is right; anything
// else is a cost with nothing to show for it.
func correctAnswer(a string) bool {
	l := strings.ToLower(a)
	spec := strings.Contains(l, "sdd-order") && (strings.Contains(l, "rn-01") || strings.Contains(l, "cancelado não pode ser faturado"))
	code := strings.Contains(l, "order_service.go") && strings.Contains(a, "Bill")
	return spec && code
}

// setGuard puts the fixture in a guard mode, the way `gofi guard` would.
func setGuard(t *testing.T, proj, mode string) {
	t.Helper()
	path := filepath.Join(proj, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AI.Guard = mode
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := installAgentSettings(proj, config.AI{Guard: mode}); err != nil {
		t.Fatal(err)
	}
}

// classify names a tool call for the report and says whether it was a gofi
// retrieval and whether it was a search at all.
func classify(c toolCall) (label string, isGofi, isSearch bool) {
	switch c.Name {
	case "Bash":
		cmd, _ := c.Input["command"].(string)
		// A command is often a chain (cd … && gofi find …): any link that runs
		// gofi makes the call a gofi retrieval.
		for _, part := range strings.FieldsFunc(cmd, func(r rune) bool { return r == '&' || r == ';' || r == '|' }) {
			fields := strings.Fields(part)
			if len(fields) >= 2 && filepath.Base(fields[0]) == "gofi" {
				return "gofi " + fields[1], true, true
			}
		}
		short := strings.Join(strings.Fields(cmd), " ")
		if len(short) > 40 {
			short = short[:40] + "…"
		}
		return "Bash(" + short + ")", false, false
	case "Grep", "Glob":
		return c.Name, false, true
	case "mcp__gofi__find", "mcp__gofi__show", "mcp__gofi__path":
		return "gofi " + strings.TrimPrefix(c.Name, "mcp__gofi__") + " (mcp)", true, true
	case "Read":
		p, _ := c.Input["file_path"].(string)
		if c.Input["offset"] != nil || c.Input["limit"] != nil {
			return "Read(range) " + filepath.Base(p), false, false
		}
		return "Read(full) " + filepath.Base(p), false, true
	}
	return c.Name, false, false
}

// --- report ---------------------------------------------------------------

func (r *benchReport) markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Context benchmark — %s\n\n", r.Label)
	fmt.Fprintf(&b, "Commit `%s`, %s. Produced by `GOFI_BENCH=1 go test ./internal/cli -run Bench`.\n\n", r.Commit, r.When.Format(time.RFC3339))

	b.WriteString("## What the harness puts in context\n\n| File | Loaded | Bytes | ≈ tokens |\n|---|---|--:|--:|\n")
	total := 0
	for _, s := range r.Sizes {
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", s.What, s.Loaded, s.Bytes, s.EstTokens)
		if s.Loaded == "on invocation" {
			total += s.Bytes
		}
	}
	fmt.Fprintf(&b, "| **all skills** | | **%d** | **%d** |\n\n", total, estTokens(total))

	b.WriteString("## Search (bench/golden-knowledge.md)\n\n| Engine | Document in top 1 | top 3 | top 5 | Right section in top 3 | MRR |\n|---|---|---|---|---|--:|\n")
	rows := []searchRow{r.Search}
	if r.Holdout != nil {
		rows = append(rows, *r.Holdout)
	}
	for _, s := range rows {
		pct := func(n int) string {
			return fmt.Sprintf("%d/%d (%.0f%%)", n, s.Questions, 100*float64(n)/float64(max(s.Questions, 1)))
		}
		section := "—"
		if s.Section3 > 0 {
			section = pct(s.Section3)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %.2f |\n", s.Engine, pct(s.Recall1), pct(s.Recall3), pct(s.Recall5), section, s.MRR)
	}
	b.WriteString("\n")
	if len(r.Search.Misses) > 0 {
		b.WriteString("Missed by the retrieval core (document not in top 5):\n\n")
		for _, m := range r.Search.Misses {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		b.WriteString("\n")
	}

	if len(r.Live) > 0 {
		fmt.Fprintf(&b, "## Live context, measured by Claude Code (%s)\n\n| Scenario | Context tokens | Added | Cost |\n|---|--:|--:|--:|\n", r.Model)
		for _, l := range r.Live {
			fmt.Fprintf(&b, "| %s | %d | %+d | $%.4f |\n", l.Scenario, l.Context, l.Delta, l.CostUSD)
		}
		b.WriteString("\n")
	}
	if len(r.Tasks) > 0 {
		b.WriteString("## Task: find a rule in the spec and in the code\n\n")
		b.WriteString("| Model | Guard | Correct | Went to gofi first | gofi calls | Grep/Glob | Whole-file reads | Context | Cost |\n|---|---|---|---|--:|--:|--:|--:|--:|\n")
		for _, tr := range r.Tasks {
			fmt.Fprintf(&b, "| %s | %s | %v | %v | %d | %d | %d | %d | $%.4f |\n",
				tr.Model, tr.Guard, tr.Correct, tr.GofiBefore, tr.GofiCalls, tr.GrepGlobCalls, tr.FullReads, tr.Context, tr.CostUSD)
		}
		b.WriteString("\n")
		for _, tr := range r.Tasks {
			fmt.Fprintf(&b, "**%s, guard %s** — %s\n\n> %s\n\n", tr.Model, tr.Guard, strings.Join(tr.Tools, " → "), strings.ReplaceAll(tr.Answer, "\n", "\n> "))
		}
	}

	for _, sk := range r.Skipped {
		fmt.Fprintf(&b, "_Skipped: %s._\n", sk)
	}
	return b.String()
}

func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
