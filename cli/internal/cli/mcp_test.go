package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gofi-labs/gofi/cli/internal/config"
)

// mcpSession connects a real MCP client to the server over an in-memory
// transport: the protocol is exercised end to end, only the pipe is fake.
func mcpSession(t *testing.T, root string) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(root, config.LanguageGo)
	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(t.Context(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func callText(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func TestMCPServesTheIndex(t *testing.T) {
	root := configuredProject(t)
	writeFile(t, filepath.Join(root, "specs", "order", "sdd-order.md"), "---\ncontexto: order\n---\n# Pedido\n\n## Faturamento\n\nO pedido e faturado apos a reserva.\n")
	writeFile(t, filepath.Join(root, "src", "app", "errors.go"), "package app\n\n// ErrGone says the order was removed.\nvar ErrGone = \"pedido removido\"\n")
	cs := mcpSession(t, root)

	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Name != "index" && (tool.Annotations == nil || !tool.Annotations.ReadOnlyHint) {
			t.Errorf("%s is a query and must say it is read-only", tool.Name)
		}
	}
	for _, want := range []string{"find", "show", "path", "intake", "index_status", "index"} {
		if !slices.Contains(names, want) {
			t.Errorf("tool %s missing from %v", want, names)
		}
	}

	// The code index is built by a tool call, as an agent would after an edit.
	if out, isErr := callText(t, cs, "index", map[string]any{"target": "code"}); isErr || !strings.Contains(out, "project") {
		t.Fatalf("index: %s", out)
	}
	if out, _ := callText(t, cs, "find", map[string]any{"query": "faturamento do pedido"}); !strings.Contains(out, "sdd-order.md") {
		t.Errorf("find did not reach the spec (the document index is built on first use):\n%s", out)
	}
	if out, _ := callText(t, cs, "find", map[string]any{"query": "pedido removido", "text": true}); !strings.Contains(out, "src/app/errors.go") {
		t.Errorf("text search:\n%s", out)
	}
	if out, _ := callText(t, cs, "show", map[string]any{"ref": "specs/order/sdd-order.md"}); !strings.Contains(out, "Faturamento") {
		t.Errorf("show:\n%s", out)
	}
	if out, _ := callText(t, cs, "index_status", nil); !strings.Contains(out, "code") {
		t.Errorf("status:\n%s", out)
	}

	// A document edited between two calls is seen by the next one.
	writeFile(t, filepath.Join(root, "specs", "order", "sdd-order.md"), "---\ncontexto: order\n---\n# Pedido\n\n## Estorno\n\nO estorno devolve o credito.\n")
	if out, _ := callText(t, cs, "find", map[string]any{"query": "estorno do credito"}); !strings.Contains(out, "Estorno") {
		t.Errorf("an edited document was answered from the loaded index:\n%s", out)
	}
}

func TestMCPReportsBadArguments(t *testing.T) {
	cs := mcpSession(t, configuredProject(t))
	if out, isErr := callText(t, cs, "find", map[string]any{"query": "x", "in": []string{"nowhere"}}); !isErr || !strings.Contains(out, "nowhere") {
		t.Errorf("an unknown area was not reported as a tool error: %q", out)
	}
	if _, isErr := callText(t, cs, "index", map[string]any{"target": "everything"}); !isErr {
		t.Error("an unknown target was accepted")
	}
}

// The server is registered where the project's host reads MCP servers.
func TestInstallMCPFollowsTheHost(t *testing.T) {
	for _, c := range []struct {
		hostID, file, want string
	}{
		{"claude-code", ".mcp.json", `"mcpServers"`},
		{"codex", ".codex/config.toml", "[mcp_servers.gofi]"},
		{"copilot", ".vscode/mcp.json", `"servers"`},
	} {
		root := configuredProject(t)
		path := filepath.Join(root, config.FileName)
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg.AI.Host = c.hostID
		if err := config.Save(path, cfg); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		cmd := newInstallCmd()
		cmd.SetArgs([]string{"mcp"})
		cmd.SetOut(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s: %v", c.hostID, err)
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil || !strings.Contains(string(b), c.want) {
			t.Errorf("%s: %s = %q (%v)", c.hostID, c.file, b, err)
		}
	}
}

// The real binary over real stdio: stdout must carry nothing but protocol
// frames. One stray line from anywhere in the command breaks the host, which
// only reports that the server "failed to start".
func TestMCPBinarySpeaksOnlyProtocolOnStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	root := configuredProject(t)
	bin := buildGofi(t, t.TempDir())

	cmd := exec.Command(bin, "mcp")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOFI_NO_SETUP=1", "NO_COLOR=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdoutPipe)
		sc.Buffer(make([]byte, 0, 1<<16), 1<<22)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	for _, msg := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"index_status","arguments":{}}}`,
	} {
		if _, err := io.WriteString(stdin, msg+"\n"); err != nil {
			t.Fatal(err)
		}
	}

	var all []string
	ids := map[float64]bool{}
	deadline := time.After(20 * time.Second)
	// Requests are answered concurrently, so in any order: wait for all.
	for !ids[1] || !ids[2] || !ids[3] {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("server closed stdout early\n%s", strings.Join(all, "\n"))
			}
			all = append(all, line)
			var frame map[string]any
			if err := json.Unmarshal([]byte(line), &frame); err != nil || frame["jsonrpc"] != "2.0" {
				t.Fatalf("stdout carried a non-protocol line: %q", line)
			}
			if id, ok := frame["id"].(float64); ok {
				ids[id] = true
			}
		case <-deadline:
			t.Fatalf("no answer to the tool call\nstdout: %s\nstderr: %s", strings.Join(all, "\n"), stderr.String())
		}
	}
	// Closing stdin is how a host ends the session; the server must exit.
	stdin.Close()
	for range lines {
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("server did not exit cleanly: %v\n%s", err, stderr.String())
	}
	for _, id := range []float64{1, 2} {
		if !ids[id] {
			t.Errorf("no response to request %v", id)
		}
	}
	stdout := strings.Join(all, "\n")
	if !strings.Contains(stdout, `"name":"find"`) {
		t.Errorf("tools/list did not list find:\n%s", stdout)
	}
}

// intake plans a request for an agent that is not driven by gofi — the entry
// Codex, Copilot and Cursor reach through MCP.
func TestMCPIntakePlansARequest(t *testing.T) {
	root := configuredProject(t)
	skills, _ := filepath.Glob("../../../ai/skills/*/contract.yaml")
	for _, s := range skills {
		b, _ := os.ReadFile(s)
		writeFile(t, filepath.Join(root, ".claude/skills", filepath.Base(filepath.Dir(s)), "contract.yaml"), string(b))
	}
	writeFile(t, filepath.Join(root, "specs", "pricing", "sdd-pricing.md"),
		"---\ntipo: spec\nformato: sdd\ncontexto: pricing\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [preco]\n---\n# SDD — pricing\n\n## 1. Envio de preços\n\nEm lote.\n")
	cs := mcpSession(t, root)
	out, isErr := callText(t, cs, "intake", map[string]any{"request": "altere o envio de pricing, adicionando um rate limit"})
	if isErr {
		t.Fatalf("intake: %s", out)
	}
	for _, want := range []string{"/gofi-spec", "/gofi-eng", "/gofi-qa", "Pedido montado", "pricing"} {
		if !strings.Contains(out, want) {
			t.Errorf("intake answer lacks %q:\n%s", want, out)
		}
	}
	if out, isErr := callText(t, cs, "intake", map[string]any{"request": " "}); !isErr {
		t.Errorf("an empty request must be refused: %s", out)
	}
}

// The index tool rebuilds the project the server serves, wherever the process
// was started — never a project found from its working folder.
func TestMCPIndexesTheProjectItServes(t *testing.T) {
	root := configuredProject(t)
	writeFile(t, filepath.Join(root, "src", "app", "a.go"), "package app\n\nfunc A() {}\n")
	cs := mcpSession(t, root)
	if out, isErr := callText(t, cs, "index", map[string]any{"target": "code"}); isErr {
		t.Fatalf("index: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".gofi", "index", "code")); err != nil {
		t.Errorf("the served project was not indexed: %v", err)
	}
	if cwd, _ := os.Getwd(); filepath.Base(cwd) != "cli" {
		t.Errorf("the working folder moved: %s", cwd)
	}
}
