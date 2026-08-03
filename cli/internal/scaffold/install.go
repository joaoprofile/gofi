package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	ProjectMarker = "__PROJECT__"
	RootMarker    = "__ROOT__"
	PackageMarker = "__PKG__"
	TemplateExt   = ".tmpl"
	GitkeepName   = ".gitkeep"
)

// TemplateData provides the values exposed to .tmpl files when rendering.
//
// SourceRoot is the source folder name inside the workspace (e.g. "src",
// "services", "backend"). It replaces the RootMarker in path components and
// is exposed as {{.SourceRoot}} to .tmpl files.
//
// Module is the one identifier the user supplies for the backend, spelled the
// way its ecosystem spells it: a Go module path, a Java base package, a .NET
// root namespace, an npm package name. PackagePath, GroupID and ArtifactID are
// derived from it — see moduleParts.
type TemplateData struct {
	ProjectName string
	Language    string
	Module      string
	PackagePath string // Module as a path: com.acme.svc -> com/acme/svc
	GroupID     string // Module minus its last segment, for Maven
	ArtifactID  string // Module's last segment
	SourceRoot  string
	Date        string
	AIHost      string
	AIModel     string
	Agents      []string
}

// WithModuleParts returns a copy of d with PackagePath, GroupID and ArtifactID
// derived from Module. Dotted (Java, C#) and slash-separated (Go, npm) module
// strings both work; a scoped npm name (@acme/svc) drops the sigil.
func (d TemplateData) WithModuleParts() TemplateData {
	m := strings.TrimPrefix(strings.TrimSpace(d.Module), "@")
	if m == "" {
		return d
	}
	segs := strings.FieldsFunc(m, func(r rune) bool { return r == '.' || r == '/' })
	if len(segs) == 0 {
		return d
	}
	d.PackagePath = strings.Join(segs, "/")
	d.ArtifactID = segs[len(segs)-1]
	if len(segs) > 1 {
		d.GroupID = strings.Join(segs[:len(segs)-1], ".")
	}
	return d
}

// InstallOptions tweaks how installFS handles entries.
type InstallOptions struct {
	// ExcludePrefixes: relative paths (forward slashes) starting with any of
	// these prefixes are skipped. Used by `gofi update` to preserve
	// user-managed dirs like knowledge/ and memory/.
	ExcludePrefixes []string

	// KeepExisting: files already present at the destination are left alone.
	// Adoption of an existing codebase relies on this — the scaffold fills the
	// gaps without overwriting code the user already wrote.
	KeepExisting bool
}

// installFS walks srcFS rooted at root, substitutes the ProjectMarker in
// path components with data.ProjectName, renders .tmpl files via text/template
// (and strips the suffix), and copies everything else verbatim. .gitkeep
// files are dropped (they only exist to preserve empty dirs in embed.FS).
//
// Returns the absolute paths of all files created. Caller is responsible for
// rollback on error (typically os.RemoveAll on the project root).
func installFS(srcFS fs.FS, root, dest string, data TemplateData, opts InstallOptions) ([]string, error) {
	var created []string

	walkErr := fs.WalkDir(srcFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}

		rel := strings.TrimPrefix(p, root+"/")
		if hasAnyPrefix(rel, opts.ExcludePrefixes) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel = strings.ReplaceAll(rel, ProjectMarker, data.ProjectName)
		if data.SourceRoot != "" {
			rel = strings.ReplaceAll(rel, RootMarker, data.SourceRoot)
		}
		if data.PackagePath != "" {
			rel = strings.ReplaceAll(rel, PackageMarker, data.PackagePath)
		}
		target := filepath.Join(dest, rel)

		if d.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			return nil
		}

		if filepath.Base(target) == GitkeepName {
			return nil
		}

		raw, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}

		content := raw
		if strings.HasSuffix(target, TemplateExt) {
			target = strings.TrimSuffix(target, TemplateExt)
			tmpl, err := template.New(filepath.Base(p)).Parse(string(raw))
			if err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("render %s: %w", p, err)
			}
			content = buf.Bytes()
		}

		if opts.KeepExisting {
			if _, statErr := os.Stat(target); statErr == nil {
				return nil
			}
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		created = append(created, target)
		return nil
	})

	return created, walkErr
}

func hasAnyPrefix(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		pre = strings.TrimSuffix(pre, "/")
		if p == pre || strings.HasPrefix(p, pre+"/") {
			return true
		}
	}
	return false
}
