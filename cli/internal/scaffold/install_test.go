package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func sampleData() TemplateData {
	return TemplateData{
		ProjectName: "my-svc",
		Language:    "go",
		GoModule:    "github.com/acme/my-svc",
		SourceRoot:  "src",
		Date:        "2026-04-25",
		AIHost:      "claude-vscode",
		AIModel:     "claude-opus-4-7",
		Agents:      []string{"gofi-pd", "gofi-spec", "gofi-eng", "gofi-qa"},
	}
}

func TestInstallGo(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallGo(dir, sampleData()); err != nil {
		t.Fatalf("InstallGo: %v", err)
	}

	mustExist(t, dir,
		".gitignore",
		"README.md",
		"go.work",
		"src/go.mod",
		"src/my-svc/main.go",
	)

	mustContain(t, filepath.Join(dir, "src/go.mod"), "module github.com/acme/my-svc")
	mustContain(t, filepath.Join(dir, "src/my-svc/main.go"), "my-svc")
	mustContain(t, filepath.Join(dir, "README.md"), "my-svc")

	// .gitkeep should not be copied
	for _, p := range []string{"src/.migrations/.gitkeep", "src/domain/.gitkeep"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("%s should not be copied to dest", p)
		}
	}
	// but the directories should exist
	mustExist(t, dir, "src/.migrations", "src/domain")
}

func TestInstallRust(t *testing.T) {
	dir := t.TempDir()
	data := sampleData()
	data.Language = "rust"
	if _, err := InstallRust(dir, data); err != nil {
		t.Fatalf("InstallRust: %v", err)
	}

	mustExist(t, dir,
		".gitignore",
		"README.md",
		"Cargo.toml",
		"crates/my-svc/Cargo.toml",
		"crates/my-svc/src/main.rs",
	)
	mustContain(t, filepath.Join(dir, "Cargo.toml"), `members = ["crates/my-svc"]`)
	mustContain(t, filepath.Join(dir, "crates/my-svc/Cargo.toml"), `name = "my-svc"`)
}

// fixtureFS returns an in-memory gofi monorepo tree mirroring the layout
// expected by InstallAgentsContent / InstallSDKContent. All harness content
// lives under ai/. Tiny on purpose.
func fixtureFS() fs.FS {
	return fstest.MapFS{
		"ai/skills/gofi-pd.md":                     {Data: []byte("# pd skill")},
		"ai/skills/gofi-spec.md":                   {Data: []byte("# spec skill")},
		"ai/skills/gofi-eng.md":                    {Data: []byte("# eng skill")},
		"ai/skills/gofi-qa.md":                     {Data: []byte("# qa skill")},
		"ai/claude/CLAUDE.md":                      {Data: []byte("# CLAUDE")},
		"ai/templates/sdd-template.md":             {Data: []byte("# SDD")},
		"ai/templates/prd-template.md":             {Data: []byte("# PRD")},
		"ai/memory/project.md.tmpl":                {Data: []byte("# Memory — {{.ProjectName}}")},
		"ai/knowledge/shared/memory-protocol.md":   {Data: []byte("# memory protocol")},
		"ai/knowledge/shared/learning-protocol.md": {Data: []byte("# learning protocol")},
		"ai/knowledge/shared/ddd-principles.md":    {Data: []byte("# ddd principles")},
		"ai/knowledge/eng/rbac-helper.md":          {Data: []byte("# rbac helper")},
		"ai/knowledge/ui/design-tokens.md":         {Data: []byte("# design tokens")},
		"ai/institutional/INDEX.md":                 {Data: []byte("# Institutional Index — {{NOME_DO_PRODUTO}}")},
		"ai/institutional/README.md":               {Data: []byte("# Institutional — {{NOME_DO_PRODUTO}}")},
		"ai/institutional/domain.md":               {Data: []byte("# Domain — {{NOME_DO_PRODUTO}}\ntopics: [{{METRICA_1}}]")},
		"ai/sdk/go/boilerplates/model.md":          {Data: []byte("model boilerplate")},
		"ai/sdk/go/sdk-docs/overview.md":           {Data: []byte("sdk overview")},
		"ai/sdk/go/knowledge/error-handling.md":    {Data: []byte("knowledge error handling")},
	}
}

func installAgentsFromFixture(t *testing.T, root string, data TemplateData) {
	t.Helper()
	fsys := fixtureFS()
	if _, err := InstallAgentsContent(fsys, ".", root, data, InstallNew); err != nil {
		t.Fatalf("InstallAgentsContent: %v", err)
	}
	if data.Language != "" {
		if _, err := InstallSDKContent(fsys, "ai/sdk/"+data.Language, root, data.Language); err != nil {
			t.Fatalf("InstallSDKContent: %v", err)
		}
	}
}

func TestInstallAgentsContent_AllAgents(t *testing.T) {
	dir := t.TempDir()
	installAgentsFromFixture(t, dir, sampleData())
	mustExist(t, dir,
		".claude/CLAUDE.md",
		".claude/skills/gofi-pd.md",
		".claude/skills/gofi-spec.md",
		".claude/skills/gofi-eng.md",
		".claude/skills/gofi-qa.md",
		".claude/templates/sdd-template.md",
		".claude/memory/project.md",
		".claude/knowledge/shared",
		".claude/knowledge/pd",
		".claude/knowledge/spec",
		".claude/knowledge/eng",
		".claude/knowledge/qa",
	)
	mustContain(t, filepath.Join(dir, ".claude/memory/project.md"), "my-svc")
	mustExist(t, dir,
		".claude/knowledge/shared/memory-protocol.md",
		".claude/knowledge/shared/learning-protocol.md",
		".claude/knowledge/shared/ddd-principles.md",
	)
}

func TestInstallAgentsContent_UpdatePreservesSharedKnowledge(t *testing.T) {
	dir := t.TempDir()
	fsys := fixtureFS()
	if _, err := InstallAgentsContent(fsys, ".", dir, sampleData(), InstallNew); err != nil {
		t.Fatalf("InstallAgentsContent (new): %v", err)
	}

	// Simulate team edits in shared/ and a brand-new file.
	editedPath := filepath.Join(dir, ".claude/knowledge/shared/memory-protocol.md")
	if err := os.WriteFile(editedPath, []byte("# team-edited"), 0o644); err != nil {
		t.Fatalf("write edited: %v", err)
	}
	teamPath := filepath.Join(dir, ".claude/knowledge/shared/team-glossary.md")
	if err := os.WriteFile(teamPath, []byte("# team only"), 0o644); err != nil {
		t.Fatalf("write team file: %v", err)
	}

	if _, err := InstallAgentsContent(fsys, ".", dir, sampleData(), InstallUpdate); err != nil {
		t.Fatalf("InstallAgentsContent (update): %v", err)
	}

	mustContain(t, editedPath, "team-edited")
	mustContain(t, teamPath, "team only")
}

func TestInstallAgentsContent_FilterAgents(t *testing.T) {
	dir := t.TempDir()
	data := sampleData()
	data.Agents = []string{"gofi-spec", "gofi-eng"}
	installAgentsFromFixture(t, dir, data)
	// All skills are always installed, regardless of the selected agent set.
	for _, kept := range []string{"gofi-pd.md", "gofi-spec.md", "gofi-eng.md", "gofi-qa.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".claude/skills", kept)); err != nil {
			t.Errorf("expected %s to be installed: %v", kept, err)
		}
	}
	// All upstream knowledge is seeded regardless of selection: ui/ ships
	// content even though gofi-ui is not selected here.
	mustExist(t, dir,
		".claude/knowledge/shared/memory-protocol.md",
		".claude/knowledge/eng/rbac-helper.md",
		".claude/knowledge/ui/design-tokens.md",
	)
	// A selected agent without upstream content still gets an empty placeholder.
	if _, err := os.Stat(filepath.Join(dir, ".claude/knowledge/spec")); err != nil {
		t.Errorf("expected placeholder knowledge/spec for selected agent: %v", err)
	}
	// Unselected agents without upstream content get nothing.
	for _, dropped := range []string{"pd", "qa"} {
		if _, err := os.Stat(filepath.Join(dir, ".claude/knowledge", dropped)); !os.IsNotExist(err) {
			t.Errorf("expected knowledge/%s NOT to exist for unselected agent (got err=%v)", dropped, err)
		}
	}
}

func TestInstallAgentsContent_InstitutionalSeed(t *testing.T) {
	dir := t.TempDir()
	installAgentsFromFixture(t, dir, sampleData()) // ProjectName my-svc
	base := ".claude/institutional/my-svc"
	// The whole guided template is seeded (not just INDEX + README).
	mustExist(t, dir, base+"/INDEX.md", base+"/README.md", base+"/domain.md")
	// {{NOME_DO_PRODUTO}} is auto-filled with the project name.
	mustContain(t, filepath.Join(dir, base, "README.md"), "Institutional — my-svc")
	mustContain(t, filepath.Join(dir, base, "domain.md"), "Domain — my-svc")
	// Other placeholders stay literal for the team to fill during discovery.
	mustContain(t, filepath.Join(dir, base, "domain.md"), "{{METRICA_1}}")
}

func TestInstallInstitutionalSeed_FallsBackToEmbedded(t *testing.T) {
	// A source without ai/institutional/ falls back to the embedded scaffold.
	fsys := fstest.MapFS{"ai/skills/gofi-pd.md": {Data: []byte("# pd")}}
	dir := t.TempDir()
	created, err := InstallInstitutionalSeed(fsys, ".", dir, "my-svc", TemplateData{ProjectName: "my-svc"})
	if err != nil {
		t.Fatalf("InstallInstitutionalSeed: %v", err)
	}
	if len(created) == 0 {
		t.Fatal("expected embedded fallback to seed files")
	}
	mustExist(t, dir, ".claude/institutional/my-svc/INDEX.md", ".claude/institutional/my-svc/README.md")
}

func TestInstallInstitutionalSeed_EmptyNameNoop(t *testing.T) {
	dir := t.TempDir()
	created, err := InstallInstitutionalSeed(fixtureFS(), ".", dir, "", TemplateData{})
	if err != nil {
		t.Fatalf("InstallInstitutionalSeed: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("expected no files for empty project name, got %v", created)
	}
}

func TestPathPlaceholderSubstitution(t *testing.T) {
	dir := t.TempDir()
	data := sampleData()
	data.ProjectName = "weird-name-99"
	if _, err := InstallGo(dir, data); err != nil {
		t.Fatalf("InstallGo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src/weird-name-99/main.go")); err != nil {
		t.Fatalf("expected src/weird-name-99/main.go: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src/__PROJECT__")); !os.IsNotExist(err) {
		t.Fatalf("__PROJECT__ should not appear in dest")
	}
}

func TestInstallGo_CustomSourceRoot(t *testing.T) {
	dir := t.TempDir()
	data := sampleData()
	data.SourceRoot = "services"
	if _, err := InstallGo(dir, data); err != nil {
		t.Fatalf("InstallGo: %v", err)
	}

	// Files land under <SourceRoot>/, not src/.
	mustExist(t, dir,
		"go.work",
		"services/go.mod",
		"services/my-svc/main.go",
		"services/domain",
		"services/.migrations",
	)
	if _, err := os.Stat(filepath.Join(dir, "src")); !os.IsNotExist(err) {
		t.Fatalf("src/ should not exist when SourceRoot=services")
	}
	if _, err := os.Stat(filepath.Join(dir, "__ROOT__")); !os.IsNotExist(err) {
		t.Fatalf("__ROOT__ marker should not leak into dest")
	}
	mustContain(t, filepath.Join(dir, "go.work"), "use ./services")
}

func mustExist(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
}

func mustContain(t *testing.T, file, substr string) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	if !strings.Contains(string(b), substr) {
		t.Errorf("%s does not contain %q\ngot:\n%s", file, substr, string(b))
	}
}
