package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/intake"
)

// EnvConducting is set by gofi chat and gofi ask for the Claude Code they
// drive: those turns are phases of a plan already made, and the intake's
// prompt hook stays quiet on them.
const EnvConducting = "GOFI_CONDUCTING"

// hintEvidence is how many evidence pointers the prompt hook hands over: the
// first few, which the plan's first phase reads first.
const hintEvidence = 5

// promptInput is the part of Claude Code's UserPromptSubmit input the hook
// reads.
type promptInput struct {
	Prompt     string `json:"prompt"`
	Cwd        string `json:"cwd"`
	Transcript string `json:"transcript_path"`
}

// runHookIntake is `gofi hook intake`, run by Claude Code on every prompt
// unless ai.intake is off (0001, D11): the rules plan the prompt — no model,
// no token — and the plan, the open questions and the evidence reach the agent
// as context. In gate mode a prompt with open questions is held back instead:
// the questions are shown to the person before any model reads the prompt,
// which is where a vague request costs the most. The hook cannot pick the
// session's model; when the session runs on a deep model and the plan needs
// less, it says so. Quiet on anything it cannot help with, and on any error of
// its own: the hook never stops a prompt it could not plan.
func runHookIntake(stdin io.Reader, stdout io.Writer) {
	if os.Getenv(EnvConducting) != "" {
		return
	}
	var in promptInput
	if b, err := io.ReadAll(stdin); err != nil || json.Unmarshal(b, &in) != nil {
		return
	}
	text := strings.TrimSpace(in.Prompt)
	if text == "" || strings.HasPrefix(text, "/") || strings.HasPrefix(text, "!") {
		return
	}
	root := projectRootFrom(in.Cwd)
	if root == "" {
		return
	}
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil || cfg.AI.IntakeMode() == config.IntakeOff {
		return
	}
	res, err := intake.Run(root, text, intake.Options{Language: backendLanguage(cfg)})
	if err != nil {
		return
	}
	tip := tierTip(res, sessionModel(in.Transcript))
	if cfg.AI.IntakeMode() == config.IntakeGate && res.Ask {
		reason := gateReason(res)
		if tip != "" {
			reason += "\n\n" + tip
		}
		out, _ := json.Marshal(map[string]any{"decision": "block", "reason": reason})
		_, _ = stdout.Write(append(out, '\n'))
		return
	}
	hint := intakeHint(res)
	if hint == "" {
		return
	}
	reply := map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":     "UserPromptSubmit",
		"additionalContext": hint,
	}}
	if tip != "" {
		reply["systemMessage"] = tip
	}
	out, _ := json.Marshal(reply)
	_, _ = stdout.Write(append(out, '\n'))
}

// gateReason is what the person reads when gate holds a prompt back: the
// questions, with their options, and the ways to go on.
func gateReason(r *intake.Result) string {
	var b strings.Builder
	b.WriteString("gofi: o pedido ficou em aberto — nada foi enviado ao modelo.\n")
	for _, q := range r.Questions {
		b.WriteString("\n• " + q.Text)
		if len(q.Options) > 0 {
			b.WriteString("  [" + strings.Join(q.Options, "] [") + "]")
		}
	}
	b.WriteString("\n\nReescreva o pedido com a resposta, ou responda pelo terminal: gofi ask \"" + r.Request + "\"")
	b.WriteString("\nPara enviar mesmo assim, invoque a skill direto (ex.: /gofi-eng ...).")
	return b.String()
}

// tierRank orders the tiers, light lowest.
var tierRank = map[host.Tier]int{host.Light: 0, host.Standard: 1, host.Deep: 2}

// tierTip is what the person reads when the session runs on a deep model and
// the plan needs no phase that deep: each skill still runs on its own tier's
// model, but a turn outside them runs on the session's.
func tierTip(r *intake.Result, model string) string {
	if model == "" || len(r.Plan) == 0 || !deepModel(model) {
		return ""
	}
	top := host.Light
	for _, ph := range r.Plan {
		if tierRank[ph.Tier] > tierRank[top] {
			top = ph.Tier
		}
	}
	if top == host.Deep {
		return ""
	}
	return fmt.Sprintf("gofi: o pedido é trabalho de nível %s, e a sessão está em %s. `/model sonnet` custa metade; `gofi ask` roda cada fase no nível dela.", top, model)
}

// deepModel reports whether a Claude model is of the deep tier's families.
func deepModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "opus") || strings.Contains(m, "fable")
}

// transcriptTail is how much of the transcript's end is read for the model:
// the last assistant message is there.
const transcriptTail = 256 << 10

var modelField = regexp.MustCompile(`"model":"(claude-[a-z0-9.-]+)"`)

// sessionModel is the model of the session's last answer, read from the end
// of its transcript. Empty when there is none yet — a first prompt.
func sessionModel(path string) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > transcriptTail {
		_, _ = f.Seek(st.Size()-transcriptTail, io.SeekStart)
	}
	b, _ := io.ReadAll(f)
	m := modelField.FindAllSubmatch(b, -1)
	if len(m) == 0 {
		return ""
	}
	return string(m[len(m)-1][1])
}

// intakeHint is the plan as the agent reads it with the prompt: short, since
// it rides along every one. Empty when there is nothing to plan.
func intakeHint(r *intake.Result) string {
	if r.Intent == "explain" && len(r.Evidence) > 0 && !r.Ask {
		// A question about the project: where the answer is, so the agent
		// reads those lines instead of searching the tree.
		var b strings.Builder
		b.WriteString("gofi intake (pergunta sobre o projeto — comece por estes trechos, só as linhas indicadas):\n")
		for i, e := range r.Evidence {
			if i == hintEvidence {
				break
			}
			fmt.Fprintf(&b, "- leia: %s\n", e.String())
		}
		return strings.TrimRight(b.String(), "\n")
	}
	if r.Direct != "" || (len(r.Plan) == 0 && !r.Ask && r.Answer == "") {
		return ""
	}
	var b strings.Builder
	b.WriteString("gofi intake (plano sugerido pelas regras do projeto; confirme com a pessoa se divergir do pedido):\n")
	fmt.Fprintf(&b, "- pedido: %s · %s", r.Intent, r.Artifact)
	if c := r.Context; c != nil {
		if c.New {
			fmt.Fprintf(&b, " · contexto novo %s", c.Name)
		} else {
			fmt.Fprintf(&b, " · contexto %s (tem %s)", c.Name, strings.Join(c.Has, ", "))
		}
	}
	b.WriteString("\n")
	if r.Answer != "" {
		fmt.Fprintf(&b, "- o índice responde: %s\n", r.Answer)
	}
	if len(r.Plan) > 0 {
		var phases []string
		for _, p := range r.Plan {
			phases = append(phases, fmt.Sprintf("/%s (%s)", p.Role, p.Tier))
		}
		fmt.Fprintf(&b, "- fases: %s — siga o SKILL.md de cada papel na ordem; pare para revisão depois de PRD ou spec\n", strings.Join(phases, " → "))
	}
	for _, q := range r.Questions {
		fmt.Fprintf(&b, "- pergunte antes de começar: %s", q.Text)
		if len(q.Options) > 0 {
			fmt.Fprintf(&b, " (opções: %s)", strings.Join(q.Options, ", "))
		}
		b.WriteString("\n")
	}
	var packs []string
	for _, p := range r.Packs {
		packs = append(packs, p.Dir)
	}
	if len(packs) > 0 {
		fmt.Fprintf(&b, "- especialidades: %s\n", strings.Join(packs, ", "))
	}
	for i, e := range r.Evidence {
		if i == hintEvidence {
			break
		}
		fmt.Fprintf(&b, "- leia: %s\n", e.String())
	}
	return strings.TrimRight(b.String(), "\n")
}
