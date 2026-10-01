// Package guard keeps an agent asking the index before it searches the tree.
//
// The agent is told, in its instructions and in the MCP server's, to ask gofi
// before it greps. Telling is not enough: under pressure a model reaches for
// the tool it knows. The guard is the hook that notices. It sees three kinds of
// event, all through Claude Code's documented hook input:
//
//   - a prompt from the person starts a new question, so nothing has been
//     asked of the index yet;
//   - a gofi query — find, show or path, as an MCP tool or a shell command —
//     marks the question as asked;
//   - a raw search — Grep, Glob, a whole-file Read, grep or rg in the shell —
//     before any query is what the guard is for: in warn mode the agent is
//     told, in enforce mode the call is refused.
//
// A targeted read (with a line range), a small file, a path outside the
// project: all pass. Knowing where to look is the point, and those are signs
// the agent does.
//
// The guard fails open. It advises; a bug in it must never stop an agent.
package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Input is the part of Claude Code's hook input the guard reads.
type Input struct {
	SessionID string         `json:"session_id"`
	Event     string         `json:"hook_event_name"`
	Tool      string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	Cwd       string         `json:"cwd"`
}

// Hook events the guard handles.
const (
	EventPrompt  = "UserPromptSubmit"
	EventPreTool = "PreToolUse"
)

// Kind is what a call means to the guard.
type Kind int

const (
	Other     Kind = iota // nothing the guard cares about
	NewPrompt             // a person asked something: start over
	Query                 // the index was asked
	Search                // the tree was searched directly
)

// SmallFile is the size under which reading a whole file is not a search: it
// costs less than the question that would have found the right lines.
const SmallFile = 120

// MCPPrefix is the prefix Claude Code gives the gofi MCP server's tools.
const MCPPrefix = "mcp__gofi__"

var (
	// A gofi query in a shell command, wherever it sits in a chain.
	reGofiQuery = regexp.MustCompile(`(^|[\s;&|(])(\S*/)?gofi\s+(find|show|path)\b`)
	// A raw search in a shell command: the first word of any segment.
	reShellSearch = regexp.MustCompile(`^(\S*/)?(grep|egrep|fgrep|rg|ag|ack)\b|^git\s+grep\b|^find\s`)
)

// Classify says what a hook event means, and for a search, what was searched.
// root is the project; a search outside it is not the guard's business.
func Classify(in Input, root string) (Kind, string) {
	if in.Event == EventPrompt {
		return NewPrompt, ""
	}
	if in.Event != EventPreTool {
		return Other, ""
	}
	switch in.Tool {
	case MCPPrefix + "find", MCPPrefix + "show", MCPPrefix + "path":
		return Query, ""
	case "Grep", "Glob":
		if p := str(in.ToolInput, "path"); p != "" && !inside(root, in.Cwd, p) {
			return Other, ""
		}
		pattern := str(in.ToolInput, "pattern")
		return Search, in.Tool + " " + pattern
	case "Read":
		p := str(in.ToolInput, "file_path")
		if p == "" || in.ToolInput["offset"] != nil || in.ToolInput["limit"] != nil || !inside(root, in.Cwd, p) {
			return Other, ""
		}
		if lines(resolve(in.Cwd, p)) < SmallFile {
			return Other, ""
		}
		return Search, "Read " + p
	case "Bash":
		cmd := str(in.ToolInput, "command")
		if reGofiQuery.MatchString(cmd) {
			return Query, ""
		}
		for _, seg := range segments(cmd) {
			if reShellSearch.MatchString(seg) {
				return Search, seg
			}
		}
	}
	return Other, ""
}

// segments splits a shell command at the operators that start a new command.
func segments(cmd string) []string {
	var out []string
	for _, s := range regexp.MustCompile(`&&|\|\||[;|\n]`).Split(cmd, -1) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func resolve(cwd, p string) string {
	if filepath.IsAbs(p) || cwd == "" {
		return p
	}
	return filepath.Join(cwd, p)
}

// inside reports whether p, resolved against cwd, is within root.
func inside(root, cwd, p string) bool {
	if root == "" {
		return true
	}
	rel, err := filepath.Rel(root, resolve(cwd, p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// lines counts a file's lines, or returns 0 when it cannot be read: a file
// that is not there is not a costly read.
func lines(p string) int {
	b, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "\n") + 1
}
