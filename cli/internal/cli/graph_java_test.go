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

// A Java backend has a graph: gofi installs the extractor it carries, the
// index reads the tree with the JDK's parser, and the graph stage holds a
// change to the layer rules — a resource that reaches the repository is
// refused at its symbol.
func TestAJavaTreeIsHeldToItsLayerRules(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java is not on PATH")
	}
	cfg, root := goProject(t)
	cfg.Backend = &config.Backend{Language: config.LanguageJava, Path: "src", Module: "com.acme.loja"}
	cfg.Version = config.CurrentVersion
	cfg.AI = config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus5}
	cfg.Sources = config.Sources{Agents: config.DefaultAgentsRef}
	cfg.Test = config.DefaultTestSection(config.LanguageJava, "src")
	if err := config.Save(filepath.Join(root, config.FileName), cfg); err != nil {
		t.Fatal(err)
	}
	pkg := "src/src/main/java/com/acme/loja/pontos/"
	write := func(rel, body string) { writeFile(t, filepath.Join(root, pkg, rel), body) }
	write("PontosRepository.java", "package com.acme.loja.pontos;\n\npublic class PontosRepository {\n    public long saldo(String c) { return 0; }\n}\n")
	write("PontosService.java", "package com.acme.loja.pontos;\n\npublic class PontosService {\n    private final PontosRepository repo;\n\n    public PontosService(PontosRepository repo) { this.repo = repo; }\n\n    public long saldo(String c) { return repo.saldo(c); }\n}\n")
	write("PontosResource.java", "package com.acme.loja.pontos;\n\npublic class PontosResource {\n    private final PontosService svc;\n\n    public PontosResource(PontosService svc) { this.svc = svc; }\n\n    public long get(String c) { return svc.saldo(c); }\n}\n")
	if _, err := external.Install(context.Background(), external.InstallOptions{ProjectRoot: root, Language: "java"}); err != nil {
		t.Fatal(err)
	}
	runGraphBuildIn(t, root)
	const run = "r-1"
	if err := captureBaseline(root, run); err != nil {
		t.Fatal(err)
	}
	write("PontosResource.java", "package com.acme.loja.pontos;\n\npublic class PontosResource {\n    private final PontosService svc;\n    private final PontosRepository repo;\n\n    public PontosResource(PontosService svc, PontosRepository repo) { this.svc = svc; this.repo = repo; }\n\n    public long get(String c) { return repo.saldo(c); }\n}\n")
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"code", "--no-html", "--baseline", loop.BaselineDir(run) + "/gofi_graph.json", "--diff", loop.DiffFile(run), "--report", loop.GraphReport(run)})
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "deny: resource -> repository") {
		t.Fatalf("the resource reaching the repository was not refused: %v\n%s", err, out.String())
	}
	d, err := loop.ReadDiff(root, run)
	if err != nil || d == nil || strings.Join(d.Layers, ",") != "api" || strings.Join(d.Contexts, ",") != "pontos" {
		t.Errorf("diff %+v, %v", d, err)
	}
}
