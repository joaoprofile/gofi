package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/aiclient"
	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/train"
)

// maybeInvokeAgents calls the AI host CLI once per agent affected by the
// training change. Failures are surfaced as warnings — they never roll back
// the persistence step.
//
// Resolution order for the "should I invoke?" decision:
//  1. explicit --no-invoke flag → skip
//  2. cfg.Training.AutoInvoke == false → skip
//  3. otherwise invoke
func maybeInvokeAgents(cfg *config.GofiConfig, scope string, topics []string, noInvoke bool) {
	if noInvoke {
		return
	}
	if cfg.Training.AutoInvoke != nil && !*cfg.Training.AutoInvoke {
		return
	}

	cli, err := aiclient.ForHost(cfg.AI.Host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot invoke ai host: %v\n", err)
		return
	}
	if !cli.Available() {
		fmt.Fprintf(os.Stderr, "warning: %s CLI not on PATH; the agent will pick up the new content on the next manual invocation\n", cli.Name())
		return
	}

	agents := agentsForScope(scope, cfg.Agents)
	if len(agents) == 0 {
		return
	}

	for _, agent := range agents {
		prompt := buildTrainPrompt(agent, scope, topics)
		invokeOne(cli, agent, prompt)
	}
}

// agentsForScope decides who to invoke. For shared scope, every active agent
// reads the new file, so each one is invoked. For per-agent scopes, only the
// owner.
func agentsForScope(scope string, activeAgents []string) []string {
	if scope == train.ScopeShared {
		return append([]string(nil), activeAgents...)
	}
	for agent, sc := range train.AgentToScope {
		if sc == scope {
			return []string{agent}
		}
	}
	return nil
}

// buildTrainPrompt produces the prompt the agent receives. Lists the topics
// affected and asks the agent to read, summarise and flag inconsistencies.
func buildTrainPrompt(agent, scope string, topics []string) string {
	var b strings.Builder
	b.WriteString("/")
	b.WriteString(agent)
	b.WriteString("\n\n")

	if len(topics) == 1 {
		fmt.Fprintf(&b, "Acabei de adicionar um arquivo de treinamento em `.claude/knowledge/%s/%s.md`.\n\n", scope, topics[0])
	} else {
		fmt.Fprintf(&b, "Acabei de adicionar %d arquivos de treinamento:\n\n", len(topics))
		for _, t := range topics {
			fmt.Fprintf(&b, "- `.claude/knowledge/%s/%s.md`\n", scope, t)
		}
		b.WriteString("\n")
	}

	b.WriteString("Por favor:\n")
	b.WriteString("1. Leia o(s) conteúdo(s)\n")
	b.WriteString("2. Confirme em uma linha o que entendeu de cada um\n")
	b.WriteString("3. Indique inconsistências ou pontos que precisam ser esclarecidos\n\n")
	b.WriteString("Trate o conteúdo como verdade do domínio adicionada pelo usuário.\n")
	return b.String()
}

func invokeOne(cli aiclient.Client, agent, prompt string) {
	fmt.Printf("\n→ invoking /%s …\n", agent)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := cli.Invoke(ctx, prompt, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "warning: invoke /%s failed: %v\n", agent, err)
	}
}
