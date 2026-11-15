package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

// The findings are read in English here: their sentences are what is tested.
func TestMain(m *testing.M) {
	i18n.SetLanguage(i18n.LangEN)
	os.Exit(m.Run())
}

func put(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func call(l *Ledger, id, name string, input map[string]any, result string) {
	raw, _ := json.Marshal(input)
	l.Call(id, name, raw)
	l.Result(id, result, false)
}

func texts(fs []Finding) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Text)
	}
	return strings.Join(out, "\n")
}

// A retrieval call is what brings text into the context: a read, a search, a
// query of the index — as a shell command or through gofi's MCP server. The
// index being rebuilt, and any other tool, is not one.
func TestWhatCountsAsRetrieval(t *testing.T) {
	root := t.TempDir()
	l := New(root)
	call(l, "1", "Read", map[string]any{"file_path": filepath.Join(root, "internal/a.go")}, strings.Repeat("x", 4000))
	call(l, "2", "Read", map[string]any{"file_path": "specs/user/spec.md", "offset": 10, "limit": 40}, strings.Repeat("x", 800))
	call(l, "3", "Grep", map[string]any{"pattern": "func New"}, strings.Repeat("x", 2000))
	call(l, "4", "Bash", map[string]any{"command": "cd x && gofi show user.Service | head -40"}, "internal/user/service.go:12\n")
	call(l, "5", "mcp__gofi__find", map[string]any{"query": "password reset"}, "specs/user/spec.md:40\n")
	call(l, "6", "Bash", map[string]any{"command": "gofi index code"}, "built")
	call(l, "7", "Edit", map[string]any{"file_path": "internal/a.go"}, "ok")
	call(l, "8", "Read", map[string]any{"file_path": ".gofi/index/code/gofi_graph_report.md"}, "report")

	rows := l.Rows()
	if len(rows) != 6 {
		t.Fatalf("6 retrieval calls, got %d: %+v", len(rows), rows)
	}
	want := []struct {
		name, target, kind string
		scoped             bool
		tokens             int
	}{
		{"Read", "internal/a.go", kindCode, false, 1000},
		{"Read", "specs/user/spec.md", kindDoc, true, 200},
		{"Grep", "func New", kindCode, false, 500},
		{graphRow, "show user.Service", kindGraph, true, 7},
		{graphRow, "find password reset", kindGraph, true, 6},
		{"Read", ".gofi/index/code/gofi_graph_report.md", kindGraph, false, 2},
	}
	for i, w := range want {
		got := rows[i]
		if got.Name != w.name || got.Target != w.target || got.Kind != w.kind || got.Scoped != w.scoped || got.Tokens != w.tokens {
			t.Errorf("row %d = %+v, want %+v", i, got, w)
		}
	}
	r := l.Report()
	if r.Calls != 6 || r.Graph != 3 || r.Code != 2 || r.Docs != 1 || r.Retrieved != 1715 {
		t.Errorf("report = %+v", r)
	}
	if r.Heaviest[0].Target != "internal/a.go" {
		t.Errorf("the heaviest call comes first: %+v", r.Heaviest)
	}
}

// Each finding fires on what was measured: a long document read whole, a
// file read twice, a call that filled the context, code hunted through the
// tree with the graph on disk.
func TestTheFindingsNameWhatCost(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gofi/index/code/gofi_graph.json", "{}")
	l := New(root)
	call(l, "1", "Read", map[string]any{"file_path": "specs/billing/spec.md"}, strings.Repeat("x", 24000))
	call(l, "2", "Read", map[string]any{"file_path": "internal/a.go"}, strings.Repeat("x", 2000))
	call(l, "3", "Read", map[string]any{"file_path": "internal/a.go"}, strings.Repeat("x", 2000))
	call(l, "4", "Grep", map[string]any{"pattern": "Invoice"}, strings.Repeat("x", 20000))
	call(l, "5", "Glob", map[string]any{"pattern": "**/*.go"}, strings.Repeat("x", 400))
	l.Usage(100, 50, 9000, 900)
	for range 3 {
		l.Turn()
	}
	got := texts(l.Report().Findings)
	for _, want := range []string{
		"specs/billing/spec.md read whole (~6k tokens)",
		"internal/a.go read 2× (~1k tokens in all)",
		"Grep Invoice brought ~5k tokens at once",
		"90% of the input came from the cache.",
		"4 code searches (~6.1k tokens) went through Grep/Glob/Read and none through the graph",
		"1 of 5 searches were aimed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Read specs/billing/spec.md brought") {
		t.Error("a document read whole is named once, not also as a heavy call")
	}

	// A cache still being written is not a problem; one that stays low is.
	young := New(root)
	young.Usage(900, 10, 100, 9000)
	young.Turn()
	if got := texts(young.Report().Findings); !strings.Contains(got, "expected in the first turns") {
		t.Errorf("a young conversation's cache is warming:\n%s", got)
	}
	for range 3 {
		young.Turn()
	}
	if got := texts(young.Report().Findings); !strings.Contains(got, "Only 1% of the input came from the cache after 4 turns") {
		t.Errorf("a cache that stays low is a warning:\n%s", got)
	}

	// Without a graph there is nothing to prefer over a grep.
	bare := New(t.TempDir())
	for i := range 4 {
		call(bare, fmt.Sprint(i), "Grep", map[string]any{"pattern": "x"}, "y")
	}
	if got := texts(bare.Report().Findings); strings.Contains(got, "graph") {
		t.Errorf("a project with no graph is not told to use one:\n%s", got)
	}
}

// What the index answered is measured in what it spared: the files it named,
// sized off disk — under the scope they were scanned in —, less the ones the
// conversation opened anyway.
func TestTheGraphIsCreditedWithWhatItSpared(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gofi/index/code/gofi_graph_index.json", `{"scopes":[{"root":"backend"},{"root":"."}]}`)
	put(t, root, "backend/internal/user/service.go", strings.Repeat("x", 8000))
	put(t, root, "backend/internal/user/repo.go", strings.Repeat("x", 4000))
	put(t, root, "docs/readme.md", strings.Repeat("x", 400))
	l := New(root)
	call(l, "1", "Bash", map[string]any{"command": "gofi show user.Service"},
		"defined internal/user/service.go:12\ncalled from internal/user/repo.go:40\nand internal/user/repo.go:88\nmissing/file.go:1\ndocs/readme.md:3\n")
	call(l, "2", "Read", map[string]any{"file_path": "backend/internal/user/repo.go", "offset": 30, "limit": 20}, "x")

	r := l.Report()
	if r.AvoidedFiles != 2 || r.AvoidedTokens != 2100 {
		t.Fatalf("spared %d files, ~%d tokens; want the service and the readme, 2100", r.AvoidedFiles, r.AvoidedTokens)
	}
	if got := texts(r.Findings); !strings.Contains(got, "The graph named 2 file(s) that did not have to be opened: ~2.1k tokens") {
		t.Errorf("findings:\n%s", got)
	}
}

func long(lines int, head string) string {
	return head + strings.Repeat("a line of the document\n", lines)
}

// The documents are checked for what the protocol depends on. One that was
// read is reported as what is costing now; the scan finds the others before
// they cost. Each comes with the one request that fixes it.
func TestTheDocumentsAreCheckedAgainstTheProtocol(t *testing.T) {
	root := t.TempDir()
	put(t, root, "specs/billing/spec.md", long(200, "# Billing\n"))
	put(t, root, "specs/user/spec.md", long(200, "---\nkeywords: [user, login]\n---\n# User\n"))
	put(t, root, "specs/order/spec.md", long(800, "---\nkeywords: order\n---\n## Scope\n## Rules\n"))
	put(t, root, "specs/small/spec.md", long(20, "# Small\n"))
	put(t, root, "prd/checkout.md", long(150, "---\ntitle: Checkout\n---\n## Problem\n"))
	put(t, root, "prd/INDEX.md", "index")
	put(t, root, ".claude/memory/contexts/user.md", long(130, "notes\n"))

	l := New(root)
	call(l, "1", "Read", map[string]any{"file_path": "specs/billing/spec.md"}, "x")
	call(l, "2", "Read", map[string]any{"file_path": "specs/order/spec.md"}, "x")
	call(l, "3", "Read", map[string]any{"file_path": "specs/small/spec.md"}, "x")
	read := Review(root, l.Rows())
	got := texts(read)
	for _, want := range []string{
		"specs/billing/spec.md (201 lines) has no front matter — without it the agent cannot tell",
		"specs/order/spec.md has 805 lines in 2 sections and was read whole",
		"specs/INDEX.md does not exist — discovery in specs/ starts by opening files",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("review misses %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "small") {
		t.Errorf("a short document is left alone:\n%s", got)
	}
	for _, f := range read {
		if f.Fix == nil || f.Fix.Label == "" || !strings.Contains(f.Fix.Prompt, "\n") {
			t.Errorf("a finding about a document carries the request that fixes it: %+v", f)
		}
	}
	if p := read[0].Fix.Prompt; !strings.Contains(p, "`specs/billing/spec.md`") || !strings.Contains(p, ".claude/expertise/harness-protocols/rag-retrieval.md") || !strings.Contains(p, "Do not change the document's content") {
		t.Errorf("the fix names the file, the protocol and its limit:\n%s", p)
	}

	// The scan finds the rest, and does not repeat what the review said.
	scan := texts(Scan(root, read))
	for _, want := range []string{
		"specs/user/spec.md (204 lines) has no `## ` sections",
		"prd/checkout.md (154 lines) has no `keywords` in its front matter",
		".claude/memory/contexts/user.md (131 lines) has no front matter",
	} {
		if !strings.Contains(scan, want) {
			t.Errorf("scan misses %q in:\n%s", want, scan)
		}
	}
	if strings.Contains(scan, "billing") || strings.Contains(scan, "specs/INDEX.md") || strings.Contains(scan, "prd/INDEX.md") {
		t.Errorf("the scan repeats the review, or asks for an index that exists:\n%s", scan)
	}
	if got := Scan(t.TempDir(), nil); len(got) != 0 {
		t.Errorf("a project with no documents has nothing to fix: %+v", got)
	}
}
