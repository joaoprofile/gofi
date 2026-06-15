package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/train"
	"github.com/joaoprofile/gofi-cli/internal/tui/editor"
)

const bufferTemplate = `# %s

<!--
  This content will be installed at .claude/knowledge/%s/%s.md and read by the
  agent before every interaction.

  Paste the domain knowledge below. Save and close to install.
  Empty content (or just these comments) cancels.
-->

`

func newTrainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "train [files...]",
		Short: "Inject domain knowledge into an agent",
		Long: `Install one or more markdown files as knowledge that the chosen agent reads
before every interaction.

The content lands in .claude/knowledge/<agent>/<topic>.md (or
.claude/knowledge/shared/<topic>.md when --shared is set). The agent's skill file
is not mutated — it is configured at init time to read everything in that folder.

Without file arguments, opens $EDITOR with a template so you can paste content
directly into the CLI.`,
		Example: `gofi train -a pd ./docs/dominio-fiscal.md
gofi train -a pd ./a.md ./b.md
gofi train -a pd
gofi train --shared ./docs/glossario.md
gofi train --from-url https://intranet/personas.md -a pd`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			shared, _ := cmd.Flags().GetBool("shared")
			topic, _ := cmd.Flags().GetString("topic")
			replace, _ := cmd.Flags().GetBool("replace")
			editorOverride, _ := cmd.Flags().GetString("editor")
			fromURL, _ := cmd.Flags().GetString("from-url")
			noInvoke, _ := cmd.Flags().GetBool("no-invoke")
			return runTrain(args, agent, shared, topic, replace, editorOverride, fromURL, noInvoke)
		},
	}
	cmd.Flags().StringP("agent", "a", "", "target agent (gofi-pd, gofi-spec, gofi-eng, gofi-qa) — required unless --shared")
	cmd.Flags().Bool("shared", false, "install in shared scope (read by all agents)")
	cmd.Flags().String("topic", "", "override topic name (default: file basename)")
	cmd.Flags().Bool("replace", false, "overwrite an existing topic")
	cmd.Flags().String("editor", "", "editor command override (default: $VISUAL → $EDITOR → vi/notepad)")
	cmd.Flags().String("from-url", "", "download the file from a URL before installing")
	cmd.Flags().Bool("no-invoke", false, "skip invoking the AI host CLI after persisting (default: invoke)")

	cmd.AddCommand(newTrainListCmd(), newTrainShowCmd(), newTrainEditCmd(), newTrainRemoveCmd())
	return cmd
}

func newTrainListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed training topics",
		Long:  `Print the topics installed for one agent or for the shared scope.`,
		Example: `gofi train list -a pd
gofi train list --shared`,
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			shared, _ := cmd.Flags().GetBool("shared")
			return runTrainList(agent, shared)
		},
	}
	cmd.Flags().StringP("agent", "a", "", "target agent")
	cmd.Flags().Bool("shared", false, "list shared scope")
	return cmd
}

func newTrainShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <topic>",
		Short:   "Print the contents of an installed topic",
		Long:    `Print the markdown content of an installed topic for inspection.`,
		Example: `gofi train show -a pd dominio-fiscal`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			shared, _ := cmd.Flags().GetBool("shared")
			return runTrainShow(agent, shared, args[0])
		},
	}
	cmd.Flags().StringP("agent", "a", "", "target agent")
	cmd.Flags().Bool("shared", false, "shared scope")
	return cmd
}

func newTrainEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit <topic>",
		Short:   "Open an installed topic in $EDITOR",
		Long:    `Open the topic file in $EDITOR (or --editor); save and close to update. After saving, the agent is invoked unless --no-invoke is passed.`,
		Example: `gofi train edit -a pd dominio-fiscal`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			shared, _ := cmd.Flags().GetBool("shared")
			editorOverride, _ := cmd.Flags().GetString("editor")
			noInvoke, _ := cmd.Flags().GetBool("no-invoke")
			return runTrainEdit(agent, shared, args[0], editorOverride, noInvoke)
		},
	}
	cmd.Flags().StringP("agent", "a", "", "target agent")
	cmd.Flags().Bool("shared", false, "shared scope")
	cmd.Flags().String("editor", "", "editor command override")
	cmd.Flags().Bool("no-invoke", false, "skip invoking the AI host CLI after saving")
	return cmd
}

func newTrainRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <topic>",
		Short:   "Delete an installed topic",
		Long:    `Delete the topic file and remove its entry from .gofi.yaml.`,
		Example: `gofi train remove -a pd dominio-fiscal`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			shared, _ := cmd.Flags().GetBool("shared")
			return runTrainRemove(agent, shared, args[0])
		},
	}
	cmd.Flags().StringP("agent", "a", "", "target agent")
	cmd.Flags().Bool("shared", false, "shared scope")
	return cmd
}

func runTrain(files []string, agent string, shared bool, topicOverride string, replace bool, editorOverride, fromURL string, noInvoke bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	scope, err := resolveScope(cfg, agent, shared)
	if err != nil {
		return err
	}
	root := projectRootFromCfg(cfg)

	var installedTopics []string

	switch {
	case fromURL != "":
		topic := topicOverride
		if topic == "" {
			topic = topicFromBasename(filepath.Base(fromURL))
		}
		body, err := downloadFile(fromURL)
		if err != nil {
			return fmt.Errorf("download %s: %w", fromURL, err)
		}
		if err := installAndPersist(cfg, root, scope, topic, fromURL, body, replace); err != nil {
			return err
		}
		installedTopics = append(installedTopics, topic)

	case len(files) == 0:
		topic := topicOverride
		if topic == "" {
			fmt.Print("Topic name (slug, e.g. dominio-fiscal): ")
			var s string
			if _, err := fmt.Scanln(&s); err != nil || s == "" {
				return errors.New("topic is required for buffer mode")
			}
			topic = strings.TrimSpace(s)
		}
		if err := train.ValidateTopic(topic); err != nil {
			return err
		}
		initial := fmt.Sprintf(bufferTemplate, topic, scope, topic)
		out, err := editor.Open(initial, editorOverride, ".md")
		if err != nil {
			return err
		}
		body := stripTemplateLeftovers(out, initial)
		if strings.TrimSpace(body) == "" {
			fmt.Println("buffer empty — install cancelled.")
			return nil
		}
		if err := installAndPersist(cfg, root, scope, topic, "(stdin)", []byte(body), replace); err != nil {
			return err
		}
		installedTopics = append(installedTopics, topic)

	default:
		for _, f := range files {
			body, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("read %s: %w", f, err)
			}
			topic := topicOverride
			if topic == "" || len(files) > 1 {
				topic = topicFromBasename(filepath.Base(f))
			}
			if err := installAndPersist(cfg, root, scope, topic, f, body, replace); err != nil {
				return err
			}
			installedTopics = append(installedTopics, topic)
		}
	}

	if len(installedTopics) > 0 {
		maybeInvokeAgents(cfg, scope, installedTopics, noInvoke)
	}
	return nil
}

func installAndPersist(cfg *config.GofiConfig, root, scope, topic, source string, body []byte, replace bool) error {
	today := time.Now().Format("2006-01-02")
	item, err := train.Install(root, scope, topic, source, body, today, replace)
	if err != nil {
		return err
	}
	upsertTrainingItem(cfg, scope, item)
	if err := config.Save(config.FileName, cfg); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}
	fmt.Printf("Installed %s/%s ← %s\n", scope, topic, source)
	return nil
}

func runTrainList(agent string, shared bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	scope, err := resolveScope(cfg, agent, shared)
	if err != nil {
		return err
	}
	root := projectRootFromCfg(cfg)
	items, err := train.List(root, scope)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Printf("no topics installed in %s\n", scope)
		return nil
	}
	sort.Strings(items)
	fmt.Printf("\n  %s\n", scope)
	for _, t := range items {
		fmt.Printf("    %s\n", t)
	}
	fmt.Println()
	return nil
}

func runTrainShow(agent string, shared bool, topic string) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	scope, err := resolveScope(cfg, agent, shared)
	if err != nil {
		return err
	}
	root := projectRootFromCfg(cfg)
	body, err := train.Read(root, scope, topic)
	if err != nil {
		return fmt.Errorf("read topic: %w", err)
	}
	_, _ = os.Stdout.Write(body)
	return nil
}

func runTrainEdit(agent string, shared bool, topic, editorOverride string, noInvoke bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	scope, err := resolveScope(cfg, agent, shared)
	if err != nil {
		return err
	}
	root := projectRootFromCfg(cfg)
	body, err := train.Read(root, scope, topic)
	if err != nil {
		return fmt.Errorf("read topic: %w", err)
	}
	out, err := editor.Open(string(body), editorOverride, ".md")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		fmt.Println("buffer empty — edit cancelled.")
		return nil
	}
	// Strip any old gofi-train header before re-installing; Install adds a fresh one.
	clean := stripGofiHeader(out)
	if strings.TrimSpace(clean) == "" {
		fmt.Println("buffer reduced to header only — edit cancelled.")
		return nil
	}
	if err := installAndPersist(cfg, root, scope, topic, "(edit)", []byte(clean), true); err != nil {
		return err
	}
	maybeInvokeAgents(cfg, scope, []string{topic}, noInvoke)
	return nil
}

func runTrainRemove(agent string, shared bool, topic string) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	scope, err := resolveScope(cfg, agent, shared)
	if err != nil {
		return err
	}
	root := projectRootFromCfg(cfg)
	if err := train.Remove(root, scope, topic); err != nil {
		return err
	}
	removeTrainingItem(cfg, scope, topic)
	if err := config.Save(config.FileName, cfg); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}
	fmt.Printf("Removed %s/%s.\n", scope, topic)
	return nil
}

// resolveScope translates the (agent, shared) flags into the scope name used
// by train.* and config.Training fields. Validates that agent is active.
func resolveScope(cfg *config.GofiConfig, agent string, shared bool) (string, error) {
	if shared {
		if agent != "" {
			return "", errors.New("--agent and --shared are mutually exclusive")
		}
		return train.ScopeShared, nil
	}
	if agent == "" {
		return "", errors.New("either --agent <name> or --shared is required")
	}
	scope, ok := train.AgentToScope[agent]
	if !ok {
		return "", fmt.Errorf("unknown agent %q", agent)
	}
	if !slices.Contains(cfg.Agents, agent) {
		return "", fmt.Errorf("agent %q is not active in this project; run 'gofi agent add %s' first", agent, agent)
	}
	return scope, nil
}

func upsertTrainingItem(cfg *config.GofiConfig, scope string, item train.Item) {
	ci := config.TrainingItem{
		Topic:       item.Topic,
		Source:      item.Source,
		InstalledAt: item.InstalledAt,
		Hash:        item.Hash,
	}
	target := trainingSlicePtr(cfg, scope)
	if target == nil {
		return
	}
	for i, existing := range *target {
		if existing.Topic == item.Topic {
			(*target)[i] = ci
			return
		}
	}
	*target = append(*target, ci)
}

func removeTrainingItem(cfg *config.GofiConfig, scope, topic string) {
	target := trainingSlicePtr(cfg, scope)
	if target == nil {
		return
	}
	out := (*target)[:0]
	for _, it := range *target {
		if it.Topic != topic {
			out = append(out, it)
		}
	}
	*target = out
}

func trainingSlicePtr(cfg *config.GofiConfig, scope string) *[]config.TrainingItem {
	switch scope {
	case "shared":
		return &cfg.Training.Shared
	case "pd":
		return &cfg.Training.PD
	case "spec":
		return &cfg.Training.Spec
	case "eng":
		return &cfg.Training.Eng
	case "qa":
		return &cfg.Training.QA
	}
	return nil
}

func topicFromBasename(name string) string {
	name = strings.TrimSuffix(name, ".md")
	name = strings.ToLower(name)
	// Replace underscores and spaces with hyphens for slug-friendliness.
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func downloadFile(url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, fmt.Errorf("URL must be http(s)")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
}

// stripTemplateLeftovers removes the buffer-mode template's HTML comment
// block from the editor output if the user did not edit it.
func stripTemplateLeftovers(out, initial string) string {
	// If output equals initial verbatim → nothing was edited → return empty.
	if strings.TrimSpace(out) == strings.TrimSpace(initial) {
		return ""
	}
	// Otherwise drop only the literal comment block to avoid persisting our
	// instructions in the user's content.
	startMarker := "<!--"
	endMarker := "-->"
	for {
		i := strings.Index(out, startMarker)
		if i < 0 {
			break
		}
		j := strings.Index(out[i:], endMarker)
		if j < 0 {
			break
		}
		out = out[:i] + out[i+j+len(endMarker):]
	}
	return strings.TrimSpace(out) + "\n"
}

// stripGofiHeader removes a leading <!-- gofi-train ... --> block (if any)
// before re-installing in edit mode.
func stripGofiHeader(s string) string {
	if !strings.HasPrefix(s, train.HeaderStart) {
		return s
	}
	end := strings.Index(s, train.HeaderEnd)
	if end < 0 {
		return s
	}
	rest := s[end+len(train.HeaderEnd):]
	return strings.TrimLeft(rest, "\n")
}
