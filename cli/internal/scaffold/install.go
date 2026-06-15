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
	TemplateExt   = ".tmpl"
	GitkeepName   = ".gitkeep"
)

// TemplateData provides the values exposed to .tmpl files when rendering.
//
// SourceRoot is the source folder name inside the workspace (e.g. "src",
// "services", "backend"). It replaces the RootMarker in path components and
// is exposed as {{.SourceRoot}} to .tmpl files.
type TemplateData struct {
	ProjectName string
	Language    string
	GoModule    string
	SourceRoot  string
	Date        string
	AIHost      string
	AIModel     string
	Agents      []string
}

// InstallOptions tweaks how installFS handles entries.
type InstallOptions struct {
	// ExcludePrefixes: relative paths (forward slashes) starting with any of
	// these prefixes are skipped. Used by `gofi update` to preserve
	// user-managed dirs like knowledge/ and memory/.
	ExcludePrefixes []string
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

// installEmbedded is a thin wrapper that points installFS at the embedded
// snapshot bundled with the binary.
func installEmbedded(source, dest string, data TemplateData) ([]string, error) {
	return installFS(embeddedFS, "embedded/"+source, dest, data, InstallOptions{})
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
