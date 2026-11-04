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

// A Rust backend has a graph: gofi installs the extractor it carries, built
// with the project's cargo, and the graph stage holds a change to the layer
// rules — a handler that reaches the repository is refused at its symbol.
func TestARustTreeIsHeldToItsLayerRules(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo is not on PATH")
	}
	if testing.Short() {
		t.Skip("builds the extractor the first time")
	}
	cfg, root := goProject(t)
	cfg.Backend = &config.Backend{Language: config.LanguageRust, Path: "src"}
	cfg.Version = config.CurrentVersion
	cfg.AI = config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus5}
	cfg.Sources = config.Sources{Agents: config.DefaultAgentsRef}
	cfg.Test = config.DefaultTestSection(config.LanguageRust, "src")
	if err := config.Save(filepath.Join(root, config.FileName), cfg); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) { writeFile(t, filepath.Join(root, "src", rel), body) }
	write("src/lib.rs", "pub mod pontos;\n")
	write("src/pontos/mod.rs", "pub mod handler;\npub mod repository;\npub mod service;\n")
	write("src/pontos/repository.rs", "pub struct PontosRepository;\n\nimpl PontosRepository {\n    pub fn saldo(&self) -> i64 {\n        0\n    }\n}\n")
	write("src/pontos/service.rs", "use crate::pontos::repository::PontosRepository;\n\npub struct PontosService {\n    repo: PontosRepository,\n}\n\nimpl PontosService {\n    pub fn saldo(&self) -> i64 {\n        self.repo.saldo()\n    }\n}\n")
	write("src/pontos/handler.rs", "pub fn get() -> i64 {\n    0\n}\n")
	if _, err := external.Install(context.Background(), external.InstallOptions{ProjectRoot: root, Language: "rust"}); err != nil {
		t.Fatal(err)
	}
	runGraphBuildIn(t, root)
	const run = "r-1"
	if err := captureBaseline(root, run); err != nil {
		t.Fatal(err)
	}
	write("src/pontos/handler.rs", "use crate::pontos::repository::PontosRepository;\n\npub fn get() -> i64 {\n    PontosRepository.saldo()\n}\n")
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"code", "--no-html", "--baseline", loop.BaselineDir(run) + "/gofi_graph.json", "--diff", loop.DiffFile(run), "--report", loop.GraphReport(run)})
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "deny: handler -> repository") {
		t.Fatalf("the handler reaching the repository was not refused: %v\n%s", err, out.String())
	}
	d, err := loop.ReadDiff(root, run)
	if err != nil || d == nil || strings.Join(d.Layers, ",") != "api" {
		t.Errorf("diff %+v, %v", d, err)
	}
}
