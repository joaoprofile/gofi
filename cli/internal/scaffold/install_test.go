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
		Module:      "github.com/acme/my-svc",
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

func TestInstallGo_KeepsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	data := sampleData()

	// An existing repository already carries its own go.mod and main.go.
	if err := os.MkdirAll(filepath.Join(dir, "src/my-svc"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := map[string]string{
		"src/go.mod":         "module github.com/acme/legacy\n",
		"src/my-svc/main.go": "package main // legacy\n",
		"README.md":          "# legacy readme\n",
	}
	for rel, body := range existing {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	if _, err := InstallGo(dir, data); err != nil {
		t.Fatalf("InstallGo: %v", err)
	}

	for rel, body := range existing {
		mustContain(t, filepath.Join(dir, rel), strings.TrimSpace(body))
	}
	// Missing pieces are still filled in.
	mustExist(t, dir, "go.work", ".gitignore", "src/domain")
}

// TestInstallBackend_EveryLanguage locks the shape each scaffold produces: the
// manifest, the entrypoint and the domain/ + .migrations/ folders the gofi
// conventions expect, under the source root the user chose.
func TestInstallBackend_EveryLanguage(t *testing.T) {
	cases := []struct {
		language string
		module   string
		files    []string
		dirs     []string
		contains map[string]string
	}{
		{
			language: "go",
			module:   "github.com/acme/my-svc",
			files:    []string{"go.work", "backend/go.mod", "backend/my-svc/main.go"},
			dirs:     []string{"backend/domain", "backend/.migrations"},
			contains: map[string]string{"backend/go.mod": "module github.com/acme/my-svc"},
		},
		{
			language: "rust",
			module:   "",
			files: []string{
				"backend/Cargo.toml",
				"backend/my-svc/Cargo.toml",
				"backend/my-svc/src/main.rs",
				"backend/my-svc/src/domain/mod.rs",
			},
			dirs: []string{"backend/.migrations"},
			contains: map[string]string{
				"backend/Cargo.toml":         `members = ["my-svc"]`,
				"backend/my-svc/Cargo.toml":  `name = "my-svc"`,
				"backend/my-svc/src/main.rs": "mod domain;",
			},
		},
		{
			language: "java",
			module:   "com.acme.mysvc",
			files: []string{
				"backend/pom.xml",
				"backend/src/main/java/com/acme/mysvc/Application.java",
				"backend/src/main/resources/application.yaml",
			},
			dirs: []string{
				"backend/src/main/java/com/acme/mysvc/domain",
				"backend/src/test/java/com/acme/mysvc",
				"backend/.migrations",
			},
			contains: map[string]string{
				"backend/pom.xml": "<groupId>com.acme</groupId>",
				"backend/src/main/java/com/acme/mysvc/Application.java": "package com.acme.mysvc;",
			},
		},
		{
			language: "csharp",
			module:   "Acme.MySvc",
			files:    []string{"backend/my-svc/my-svc.csproj", "backend/my-svc/Program.cs"},
			dirs:     []string{"backend/my-svc/Domain", "backend/.migrations"},
			contains: map[string]string{
				"backend/my-svc/my-svc.csproj": "<RootNamespace>Acme.MySvc</RootNamespace>",
				"backend/my-svc/Program.cs":    "namespace Acme.MySvc;",
			},
		},
		{
			language: "nodejs",
			module:   "@acme/my-svc",
			files:    []string{"backend/package.json", "backend/tsconfig.json", "backend/src/main.ts"},
			dirs:     []string{"backend/src/domain", "backend/.migrations"},
			contains: map[string]string{"backend/package.json": `"name": "@acme/my-svc"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.language, func(t *testing.T) {
			dir := t.TempDir()
			data := sampleData()
			data.Language = tc.language
			data.Module = tc.module
			data.SourceRoot = "backend"

			if _, err := InstallBackend(tc.language, dir, data); err != nil {
				t.Fatalf("InstallBackend: %v", err)
			}

			mustExist(t, dir, append([]string{".gitignore", "README.md"}, append(tc.files, tc.dirs...)...)...)
			for rel, want := range tc.contains {
				mustContain(t, filepath.Join(dir, rel), want)
			}
			mustHaveNoMarkers(t, dir)
		})
	}
}

func TestInstallBackend_UnknownLanguageIsANoop(t *testing.T) {
	dir := t.TempDir()
	created, err := InstallBackend("cobol", dir, sampleData())
	if err != nil {
		t.Fatalf("InstallBackend: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("created %v, want nothing", created)
	}
	if HasBackendScaffold("cobol") {
		t.Error("HasBackendScaffold(cobol) = true, want false")
	}
}

// mustHaveNoMarkers fails when a path placeholder survived into the installed
// tree — a missing substitution is silent otherwise, and the project only
// breaks much later when the toolchain trips over a __ROOT__ directory.
func mustHaveNoMarkers(t *testing.T, dir string) {
	t.Helper()
	markers := []string{ProjectMarker, RootMarker, PackageMarker, TemplateExt, GitkeepName}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, m := range markers {
			if strings.Contains(d.Name(), m) {
				t.Errorf("%s: unsubstituted %q in installed tree", p, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
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
		"ai/institutional/INDEX.md":                {Data: []byte("# Institutional Index — {{NOME_DO_PRODUTO}}")},
		"ai/institutional/README.md":               {Data: []byte("# Institutional — {{NOME_DO_PRODUTO}}")},
		"ai/institutional/domain.md":               {Data: []byte("# Domain — {{NOME_DO_PRODUTO}}\nkeywords: [{{METRICA_1}}]")},
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
		".claude/skills/gofi-pd/SKILL.md",
		".claude/skills/gofi-spec/SKILL.md",
		".claude/skills/gofi-eng/SKILL.md",
		".claude/skills/gofi-qa/SKILL.md",
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
	for _, kept := range []string{"gofi-pd/SKILL.md", "gofi-spec/SKILL.md", "gofi-eng/SKILL.md", "gofi-qa/SKILL.md"} {
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

// TestInstallGo_NestedAndRootSourceRoot covers the two path shapes the wizard
// accepts beyond a single folder name: a nested path, and "." for code that
// already lives at the workspace root.
func TestInstallGo_NestedAndRootSourceRoot(t *testing.T) {
	t.Run("nested", func(t *testing.T) {
		dir := t.TempDir()
		data := sampleData()
		data.SourceRoot = "services/api"
		if _, err := InstallGo(dir, data); err != nil {
			t.Fatalf("InstallGo: %v", err)
		}
		mustExist(t, dir, "services/api/go.mod", "services/api/my-svc/main.go", "services/api/domain")
		mustContain(t, filepath.Join(dir, "go.work"), "use ./services/api")
	})
	t.Run("root", func(t *testing.T) {
		dir := t.TempDir()
		data := sampleData()
		data.SourceRoot = "."
		if _, err := InstallGo(dir, data); err != nil {
			t.Fatalf("InstallGo: %v", err)
		}
		mustExist(t, dir, "go.mod", "my-svc/main.go", "domain")
		if _, err := os.Stat(filepath.Join(dir, "src")); !os.IsNotExist(err) {
			t.Fatal("src/ should not exist when the source root is the workspace root")
		}
	})
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
