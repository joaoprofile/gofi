package sdkdoc

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ExamplesFile is the index of the SDK's runnable examples.
const ExamplesFile = "examples.md"

// Example is one runnable program the SDK ships.
type Example struct {
	Dir     string // relative to the SDK root, slash-separated
	Title   string
	Summary string
	Files   []string
}

// WriteExamples indexes the SDK's examples into outDir. It points at the
// programs where they are — the checkout, compiled and tested by the SDK's own
// CI — instead of copying them: an agent reads code that runs, at the pinned
// version, and nothing is duplicated. checkoutRel is where the checkout sits
// relative to the project root, for the paths the index prints.
func WriteExamples(srcDir, outDir, checkoutRel, version string) ([]Example, error) {
	root := filepath.Join(srcDir, "examples")
	if _, err := os.Stat(root); err != nil {
		return nil, nil
	}
	var out []Example
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err != nil {
			return nil
		}
		rel, _ := filepath.Rel(srcDir, p)
		ex := Example{Dir: filepath.ToSlash(rel)}
		ex.Title, ex.Summary = readmeLead(filepath.Join(p, "README.md"))
		entries, _ := os.ReadDir(p)
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go")) {
				ex.Files = append(ex.Files, e.Name())
			}
		}
		if len(ex.Files) > 0 {
			out = append(out, ex)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Exemplos executáveis do SDK — %s\n\n", versionNote(version))
	b.WriteString("> Gerado do SDK — não edite. Programas completos, compilados e testados na CI do SDK:\n")
	b.WriteString("> leia o código no checkout, na versão que o projeto fixou.\n\n")
	for _, ex := range out {
		title := ex.Title
		if title == "" {
			title = ex.Dir
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		fmt.Fprintf(&b, "`%s/`\n\n", under(checkoutRel, ex.Dir))
		if ex.Summary != "" {
			b.WriteString(ex.Summary + "\n\n")
		}
		for _, f := range ex.Files {
			fmt.Fprintf(&b, "- `%s`\n", under(checkoutRel, ex.Dir, f))
		}
		b.WriteString("\n")
	}
	return out, writeFile(filepath.Join(outDir, ExamplesFile), b.Bytes())
}

// readmeLead is a README's title and its first paragraph.
func readmeLead(p string) (string, string) {
	f, err := os.Open(p)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	var title string
	var para []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case title == "" && strings.HasPrefix(line, "# "):
			title = strings.TrimPrefix(line, "# ")
		case line == "":
			if len(para) > 0 {
				return title, strings.Join(para, " ")
			}
		case strings.HasPrefix(line, "#"), strings.HasPrefix(line, "```"):
			if len(para) > 0 {
				return title, strings.Join(para, " ")
			}
		default:
			if title != "" {
				para = append(para, line)
			}
		}
	}
	return title, strings.Join(para, " ")
}

// under joins a path below base, which is a project-relative directory or a
// URL — path.Join would fold the // of a scheme.
func under(base string, elem ...string) string {
	rest := path.Join(elem...)
	if strings.Contains(base, "://") {
		return strings.TrimRight(base, "/") + "/" + rest
	}
	return path.Join(base, rest)
}
