package host

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ServerName is the name gofi's MCP server is registered under, and so the
// prefix of its tools in the host (mcp__gofi__find in Claude Code).
const ServerName = "gofi"

// MCPFormat is how a host reads a project's MCP servers.
type MCPFormat struct {
	// File is the project file, relative to the root.
	File string
	// Key is where the servers sit in a JSON file ("mcpServers", "servers");
	// empty for TOML.
	Key string
}

var (
	// MCPClaude is Claude Code's .mcp.json.
	MCPClaude = MCPFormat{File: ".mcp.json", Key: "mcpServers"}
	// MCPCodex is Codex's project config, read for trusted projects only.
	MCPCodex = MCPFormat{File: ".codex/config.toml"}
	// MCPCopilot is VS Code's workspace MCP file, which Copilot reads.
	MCPCopilot = MCPFormat{File: ".vscode/mcp.json", Key: "servers"}
)

// RegisterMCP adds gofi's server to the host's project MCP file, keeping every
// other server and key as it was. It reports whether the file changed. A file
// gofi cannot parse is reported, never rewritten: it is the team's.
func (h Host) RegisterMCP(root string) (bool, error) {
	if h.MCP.Key == "" {
		return registerTOML(filepath.Join(root, filepath.FromSlash(h.MCP.File)))
	}
	return registerJSON(filepath.Join(root, filepath.FromSlash(h.MCP.File)), h.MCP.File, h.MCP.Key)
}

// Registered reports whether the host's MCP file already names gofi's server.
func (h Host) Registered(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(h.MCP.File)))
	if err != nil {
		return false
	}
	if h.MCP.Key == "" {
		return bytes.Contains(b, []byte(tomlTable))
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(b, &doc) != nil {
		return false
	}
	var servers map[string]json.RawMessage
	return json.Unmarshal(doc[h.MCP.Key], &servers) == nil && servers[ServerName] != nil
}

// jsonEntry is how a host starts the server, in the JSON formats.
var jsonEntry = map[string]any{"type": "stdio", "command": "gofi", "args": []any{"mcp"}}

func registerJSON(path, name, key string) (bool, error) {
	doc := map[string]any{}
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &doc); err != nil {
			return false, fmt.Errorf("%s: %w — fix it by hand, it is not gofi's to rewrite", name, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	servers, _ := doc[key].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if cur, ok := servers[ServerName]; ok && sameJSON(cur, jsonEntry) {
		return false, nil
	}
	servers[ServerName] = jsonEntry
	doc[key] = servers
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, append(out, '\n'), 0o644)
}

// tomlTable is gofi's server in Codex's config. A table is appended, never
// merged into existing text: TOML allows a table anywhere once, so appending is
// both valid and the smallest change to a file the team owns.
const tomlTable = "[mcp_servers." + ServerName + "]"

func registerTOML(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if bytes.Contains(b, []byte(tomlTable)) {
		return false, nil
	}
	var out strings.Builder
	out.Write(b)
	if len(b) > 0 && !bytes.HasSuffix(b, []byte("\n")) {
		out.WriteString("\n")
	}
	if len(b) > 0 {
		out.WriteString("\n")
	}
	out.WriteString(tomlTable + "\ncommand = \"gofi\"\nargs = [\"mcp\"]\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(out.String()), 0o644)
}

func sameJSON(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}
