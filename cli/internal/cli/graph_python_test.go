package cli

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// A Python backend has a graph: gofi installs the extractor it carries, the
// index reads the tree, and the graph stage holds a change to the layer
// rules — a router that reaches the repository is refused at its symbol.
func TestAPythonTreeIsHeldToItsLayerRules(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not on PATH")
	}
	cfg, root := goProject(t)
	cfg.Backend = &config.Backend{Language: config.LanguagePython, Path: "src"}
	cfg.Version = config.CurrentVersion
	cfg.AI = config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus5}
	cfg.Sources = config.Sources{Agents: config.DefaultAgentsRef}
	cfg.Test = config.DefaultTestSection(config.LanguagePython, "src")
	if err := config.Save(filepath.Join(root, config.FileName), cfg); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) { writeFile(t, filepath.Join(root, "src", rel), body) }
	write("app/__init__.py", "")
	write("app/pontos/__init__.py", "")
	write("app/pontos/repository.py", "class PontosRepository:\n    def saldo(self, cliente: str) -> int:\n        return 0\n")
	write("app/pontos/service.py", "from app.pontos.repository import PontosRepository\n\n\nclass PontosService:\n    def saldo(self, repo: PontosRepository, cliente: str) -> int:\n        return repo.saldo(cliente)\n")
	write("app/pontos/router.py", "from app.pontos.service import PontosService\n\n\ndef saldo(cliente: str) -> int:\n    return 0\n")
	if _, err := external.Install(context.Background(), external.InstallOptions{ProjectRoot: root, Language: "python"}); err != nil {
		t.Fatal(err)
	}
	runGraphBuildIn(t, root)
	const run = "r-1"
	if err := captureBaseline(root, run); err != nil {
		t.Fatal(err)
	}
	write("app/pontos/router.py", "from app.pontos import repository\nfrom app.pontos.service import PontosService\n\n\ndef saldo(cliente: str) -> int:\n    return repository.PontosRepository().saldo(cliente)\n")
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"code", "--no-html", "--baseline", loop.BaselineDir(run) + "/gofi_graph.json", "--diff", loop.DiffFile(run), "--report", loop.GraphReport(run)})
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "deny: router -> repository") {
		t.Fatalf("the router reaching the repository was not refused: %v\n%s", err, out.String())
	}
	d, err := loop.ReadDiff(root, run)
	if err != nil || d == nil || strings.Join(d.Layers, ",") != "api" || len(d.Touched) == 0 {
		t.Errorf("diff %+v, %v", d, err)
	}
}
