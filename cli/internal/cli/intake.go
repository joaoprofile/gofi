package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/intake"
	"github.com/gofi-labs/gofi/cli/internal/leancall"
)

func newIntakeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "intake <request>",
		Short: i18n.T("cmd.intake.short"),
		Long: `Read a request in free text and say how gofi would carry it out, without
calling any model: what it asks for, the context it is about and what that
context already has, the phases that take it there — each role with its tier
and why — the expertise the roles need, what is missing, and the assembled
request the first phase receives.

Everything comes from the lexicon, the index and the roles' contracts
(contract.yaml beside each skill); every decision says why. What the rules
cannot settle is a question with options taken from the index, never a guess.
A request that starts with /skill passes as it is.

What the rules leave open that the index can settle — an intent no word
named, a context among several — goes to the host's light tier through a lean
call (no project loaded, no tools, a validated answer), only then, and cached
by what it was asked. Its cost is reported. --no-model keeps to the rules.
A new context is never the model's call: it stays a question.

Two rounds at most: answer with --answer id=value (the ids the questions
carry), or --proceed to go on with each default written down as an
assumption.

--json prints the result in the gofi.intake/v1 schema, for the editor, the MCP
tool and scripts. --serve stays open for an editor that plans every message:
one request per line of stdin — {"request", "answers", "proceed", "no_model",
"tier"} — and one result per line of stdout, with the index kept loaded
between them (it is most of what planning costs) and reloaded when it changes.`,
		Example: `gofi intake "crie um PRD de sincronização de pedidos"
gofi intake "altere o fluxo de pricing, adicionando um rate limit" --json
gofi intake "corrija o cancelamento de pedidos" --tier deep
gofi intake "crie um PRD de sincronização de pedidos" --answer context=novo
gofi intake "altere as regras" --proceed`,
		Args: func(cmd *cobra.Command, args []string) error {
			if serve, _ := cmd.Flags().GetBool("serve"); serve {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			if serve, _ := cmd.Flags().GetBool("serve"); serve {
				return serveIntake(root, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			tier, _ := cmd.Flags().GetString("tier")
			if tier != "" && !slices.Contains(host.Tiers, host.Tier(tier)) {
				return fmt.Errorf("--tier %q: use light, standard or deep", tier)
			}
			opts := intake.Options{Tier: host.Tier(tier), CacheDir: filepath.Join(root, ".gofi", "cache", "intake")}
			opts.Proceed, _ = cmd.Flags().GetBool("proceed")
			answers, _ := cmd.Flags().GetStringArray("answer")
			for _, a := range answers {
				id, v, ok := strings.Cut(a, "=")
				if !ok {
					return fmt.Errorf("--answer %q: use id=value", a)
				}
				if opts.Answers == nil {
					opts.Answers = map[string]string{}
				}
				opts.Answers[strings.TrimSpace(id)] = strings.TrimSpace(v)
			}
			noModel, _ := cmd.Flags().GetBool("no-model")
			if cfg, _, err := loadProjectConfig(); err == nil {
				opts.Language = backendLanguage(cfg)
				if !noModel {
					opts.Model = lightModelFor(cfg)
				}
			}
			res, err := intake.Run(root, strings.Join(args, " "), opts)
			if err != nil {
				return err
			}
			if cfg, _, err := loadProjectConfig(); err == nil {
				fillPhaseModels(res, cfg)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), res)
			}
			printIntake(cmd.OutOrStdout(), res)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "print the result in the gofi.intake/v1 schema")
	cmd.Flags().Bool("serve", false, "stay open and plan one JSON request per line of stdin, keeping the index loaded")
	cmd.Flags().String("tier", "", "run every phase at this tier (light, standard, deep)")
	cmd.Flags().Bool("no-model", false, "keep to the rules: every open point is a question")
	cmd.Flags().StringArray("answer", nil, "answer a question of the previous round, as id=value")
	cmd.Flags().Bool("proceed", false, "go on without answering; defaults are written down as assumptions")
	return cmd
}

// lightModelFor is lightModel, indirected so tests never reach a real model.
var lightModelFor = lightModel

// lightModel is the host's light tier, reached through a lean call — or nil
// when the host has none gofi can call: the intake then keeps to its rules.
func lightModel(cfg *config.GofiConfig) intake.Model {
	h, ok := host.Get(cfg.AI.Host)
	if !ok || !host.IsClaude(h.ID) {
		return nil
	}
	return leanModel{call: leancall.Claude{Executable: os.Getenv(EnvEngine)}, model: h.Model(host.Light, cfg.AI.Tiers)}
}

// leanModel adapts the lean call to what the intake asks.
type leanModel struct {
	call  leancall.Claude
	model string
}

func (m leanModel) Decide(ctx context.Context, system, prompt string, schema []byte) ([]byte, intake.Spend, error) {
	out, sp, err := m.call.Call(ctx, leancall.Request{System: system, Prompt: prompt, Schema: schema, Model: m.model, MaxUSD: 0.05})
	return out, intake.Spend{USD: sp.USD, Millis: sp.Millis}, err
}

// printIntake renders a result for a person: what was understood, the plan
// with its reasons, and what is missing.
func printIntake(out io.Writer, r *intake.Result) {
	if r.Direct != "" {
		fmt.Fprintf(out, "  /%s — invocado direto, passa como está\n", r.Direct)
		return
	}
	fmt.Fprintf(out, "  %s · %s\n", r.Intent, r.Artifact)
	for _, s := range r.Signals {
		fmt.Fprintf(out, "    %s\n", s)
	}
	if c := r.Context; c != nil {
		state := "novo"
		if !c.New {
			state = "tem " + strings.Join(c.Has, ", ")
			if len(c.Has) == 0 {
				state = "sem PRD, spec nem código"
			}
		}
		fmt.Fprintf(out, "\n  contexto %s (%s)\n    %s\n", c.Name, state, c.Why)
	}
	if r.Answer != "" {
		fmt.Fprintf(out, "\n  resposta: %s\n", r.Answer)
	}
	if len(r.Plan) > 0 {
		fmt.Fprintln(out, "\n  plano")
		for i, p := range r.Plan {
			fmt.Fprintf(out, "    %d. /%-12s %-8s %s\n", i+1, p.Role, p.Tier, p.Why)
			fmt.Fprintf(out, "       %-21s %s\n", "", p.TierWhy)
		}
	}
	if r.Next != "" {
		fmt.Fprintf(out, "    depois: /%s\n", r.Next)
	}
	if len(r.Packs) > 0 {
		fmt.Fprintln(out, "\n  especialidades")
		for _, p := range r.Packs {
			fmt.Fprintf(out, "    %-20s %s\n", p.Pack, p.Why)
		}
	}
	if len(r.Questions) > 0 {
		fmt.Fprintln(out, "\n  falta decidir")
		for _, q := range r.Questions {
			fmt.Fprintf(out, "    [%s] %s\n", q.ID, q.Text)
			for _, o := range q.Options {
				fmt.Fprintf(out, "      - %s\n", o)
			}
		}
	}
	if r.Spent != nil {
		fmt.Fprintf(out, "\n  %s\n", r.Spent.Note)
	}
	if len(r.Questions) > 0 {
		fmt.Fprintln(out, "\n  responda com --answer <id>=<valor>, ou siga com --proceed")
	}
	for _, a := range r.Assumptions {
		fmt.Fprintf(out, "  suposição: %s\n", a)
	}
	for _, m := range r.Missing {
		fmt.Fprintf(out, "  %s\n", i18n.T("find.missing", m))
	}
}

// fillPhaseModels names, for each phase the router raised, the host's model
// for its tier — what a conductor sets for that phase. A host with no model
// per skill gets none: its phases all run on the session's model.
func fillPhaseModels(r *intake.Result, cfg *config.GofiConfig) {
	h, ok := host.Get(cfg.AI.Host)
	if !ok || !h.SkillModel {
		return
	}
	for i := range r.Plan {
		if intake.Raised(r, i) {
			r.Plan[i].Model = h.Model(r.Plan[i].Tier, cfg.AI.Tiers)
		}
		for _, more := range [][]intake.Phase{r.Plan[i].OnReject, r.Plan[i].OnBlock} {
			for j, ph := range more {
				if ph.Tier != ph.Base {
					more[j].Model = h.Model(ph.Tier, cfg.AI.Tiers)
				}
			}
		}
	}
}

// serveRequest is one line an editor sends to `gofi intake --serve`.
type serveRequest struct {
	Request string            `json:"request"`
	Answers map[string]string `json:"answers"`
	Proceed bool              `json:"proceed"`
	NoModel bool              `json:"no_model"`
	Tier    string            `json:"tier"`
}

// serveIntake plans one request per line until stdin closes, answering each
// with one line: the result, or {"error": ...}. The index stays loaded.
func serveIntake(root string, in io.Reader, out io.Writer) error {
	engines := &intake.Engines{}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		res, err := serveOne(root, line, engines)
		if err != nil {
			_ = enc.Encode(map[string]string{"error": err.Error()})
			continue
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return sc.Err()
}

func serveOne(root, line string, engines *intake.Engines) (*intake.Result, error) {
	var req serveRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if strings.TrimSpace(req.Request) == "" {
		return nil, errors.New("request: empty")
	}
	if req.Tier != "" && !slices.Contains(host.Tiers, host.Tier(req.Tier)) {
		return nil, fmt.Errorf("tier %q: use light, standard or deep", req.Tier)
	}
	opts := intake.Options{Tier: host.Tier(req.Tier), CacheDir: filepath.Join(root, ".gofi", "cache", "intake"),
		Answers: req.Answers, Proceed: req.Proceed, Engines: engines}
	// The configuration is read per request: a model or tier changed in
	// .gofi.yaml applies to the next message without restarting the editor.
	cfg, _, cfgErr := loadProjectConfig()
	if cfgErr == nil {
		opts.Language = backendLanguage(cfg)
		if !req.NoModel {
			opts.Model = lightModelFor(cfg)
		}
	}
	res, err := intake.Run(root, req.Request, opts)
	if err != nil {
		return nil, err
	}
	if cfgErr == nil {
		fillPhaseModels(res, cfg)
	}
	return res, nil
}
