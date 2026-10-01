package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/guard"
)

// claudeSettingsFile is the project's shared Claude Code settings, the file
// the team versions. Personal overrides live in settings.local.json and are
// never touched.
const claudeSettingsFile = ".claude/settings.json"

// guardCommand is how the settings run the guard. It is also how gofi finds
// its own hook entries again, to update or remove them without touching the
// team's.
const guardCommand = "gofi hook guard"

// intakeCommand is how the settings run the intake's prompt hook, and how gofi
// finds that entry again.
const intakeCommand = "gofi hook intake"

// guardMatcher is every tool the guard looks at: the raw searches it watches
// for and the gofi queries that clear them.
const guardMatcher = "Grep|Glob|Read|Bash|" + guard.MCPPrefix + ".*"

// claudeSettings is .claude/settings.json read as a generic document, so keys
// gofi does not know survive a rewrite untouched.
type claudeSettings map[string]any

func loadClaudeSettings(root string) (claudeSettings, error) {
	b, err := os.ReadFile(filepath.Join(root, claudeSettingsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return claudeSettings{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := claudeSettings{}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w — fix it by hand, it is not gofi's to rewrite", claudeSettingsFile, err)
	}
	return s, nil
}

// save writes the settings when they differ from what is on disk, and says
// whether they did.
func (s claudeSettings) save(root string) (bool, error) {
	path := filepath.Join(root, claudeSettingsFile)
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	if old, err := os.ReadFile(path); err == nil && string(old) == string(out) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}

func (s claudeSettings) object(key string) map[string]any {
	m, _ := s[key].(map[string]any)
	if m == nil {
		m = map[string]any{}
		s[key] = m
	}
	return m
}

// allowReadOnly adds the permission rules for gofi's read-only queries, and
// for the memory write an agent cannot do by itself, next to whatever the
// project already allows.
func (s claudeSettings) allowReadOnly() {
	perms := s.object("permissions")
	var allow []any
	if list, ok := perms["allow"].([]any); ok {
		allow = list
	}
	for _, rule := range append(slices.Clone(guard.ReadOnlyRules), guard.WriteRules...) {
		if !slices.Contains(allow, any(rule)) {
			allow = append(allow, rule)
		}
	}
	perms["allow"] = allow
}

// setGuard installs the guard's hooks, or removes them when on is false.
func (s claudeSettings) setGuard(on bool) {
	s.setHooks(guardCommand, map[string]string{guard.EventPreTool: guardMatcher, guard.EventPrompt: ""}, on)
}

// setIntake installs the intake's prompt hook, or removes it.
func (s claudeSettings) setIntake(on bool) {
	s.setHooks(intakeCommand, map[string]string{guard.EventPrompt: ""}, on)
}

// setHooks puts one command on the given events — event to tool matcher, ""
// for none — or takes it off. Only entries that run command are touched: the
// team's hooks, and gofi's other ones, stay as they are.
func (s claudeSettings) setHooks(command string, events map[string]string, on bool) {
	hooks := s.object("hooks")
	for event, matcher := range events {
		var kept []any
		if list, ok := hooks[event].([]any); ok {
			for _, entry := range list {
				if !entryRuns(entry, command) {
					kept = append(kept, entry)
				}
			}
		}
		if on {
			entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 10}}}
			if matcher != "" {
				entry["matcher"] = matcher
			}
			kept = append(kept, entry)
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if len(hooks) == 0 {
		delete(s, "hooks")
	}
}

// hasGuard reports whether the guard's PreToolUse hook is installed.
func (s claudeSettings) hasGuard() bool {
	hooks, _ := s["hooks"].(map[string]any)
	list, _ := hooks[guard.EventPreTool].([]any)
	return slices.ContainsFunc(list, func(e any) bool { return entryRuns(e, guardCommand) })
}

// hasIntake reports whether the intake's prompt hook is installed.
func (s claudeSettings) hasIntake() bool {
	hooks, _ := s["hooks"].(map[string]any)
	list, _ := hooks[guard.EventPrompt].([]any)
	return slices.ContainsFunc(list, func(e any) bool { return entryRuns(e, intakeCommand) })
}

// entryRuns reports whether a hook entry runs command.
func entryRuns(entry any, command string) bool {
	m, _ := entry.(map[string]any)
	hooks, _ := m["hooks"].([]any)
	for _, h := range hooks {
		if hm, _ := h.(map[string]any); hm["command"] == command {
			return true
		}
	}
	return false
}

// installAgentSettings wires a project's Claude Code to gofi: the read-only
// queries allowed, the guard in the mode ai.guard names, and the intake's
// prompt hook unless ai.intake is off.
func installAgentSettings(root string, ai config.AI) (bool, error) {
	s, err := loadClaudeSettings(root)
	if err != nil {
		return false, err
	}
	s.allowReadOnly()
	s.setGuard(ai.GuardMode() != config.GuardOff)
	s.setIntake(ai.IntakeMode() != config.IntakeOff)
	return s.save(root)
}
