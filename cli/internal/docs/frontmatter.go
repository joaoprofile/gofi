package docs

import (
	"fmt"
	"os"
	"strings"
)

// Frontmatter is the leading --- block of a gofi document, parsed flat.
//
// Deliberately not YAML: the gofi frontmatter is key: value with lists written
// [a, b, c], and a hand parser keeps the reader tolerant of the drift a real
// corpus carries. A strict YAML error would abort indexing over one bad file.
type Frontmatter map[string]string

// Parse splits a document into frontmatter and body lines. A file with no
// leading --- yields an empty Frontmatter and the whole file as body.
func Parse(content string) (Frontmatter, []string) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return Frontmatter{}, lines
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return parseFields(lines[1:i]), lines[i+1:]
		}
	}
	return Frontmatter{}, lines
}

// bodyOffset is how many lines of content come before the body: the
// frontmatter and its two --- fences, or none. Section ranges add it, so they
// count lines of the file — the numbers a reader opens it at — not of the body.
func bodyOffset(content string, body []string) int {
	total := strings.Count(strings.ReplaceAll(content, "\r\n", "\n"), "\n") + 1
	return total - len(body)
}

// ParseFile reads and parses a document.
func ParseFile(path string) (Frontmatter, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	fm, body := Parse(string(b))
	return fm, body, nil
}

func parseFields(lines []string) Frontmatter {
	fm := Frontmatter{}
	for _, ln := range lines {
		k, v, ok := strings.Cut(ln, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" || !isFieldName(k) {
			continue
		}
		fm[k] = strings.TrimSpace(v)
	}
	return fm
}

func isFieldName(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_') {
			return false
		}
	}
	return true
}

// List reads a [a, b, c] field. Missing or empty yields nil. A comment after
// the list — the templates write one (depende_de: []  # …) — is not part of it.
func (f Frontmatter) List(key string) []string {
	v := strings.TrimSpace(f[key])
	if strings.HasPrefix(v, "[") {
		if end := strings.Index(v, "]"); end >= 0 {
			v = v[:end+1]
		}
	} else {
		v = uncomment(v)
	}
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	if v == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Get returns a scalar field, with "n/a" normalized to empty since the corpus
// uses it to mean "none", and without the comment a template writes after
// some values (status: proposto  # ou em_revisao).
func (f Frontmatter) Get(key string) string {
	v := strings.Trim(uncomment(strings.TrimSpace(f[key])), `"`)
	if v == "n/a" {
		return ""
	}
	return v
}

// uncomment drops a trailing " # comment" from a scalar value.
func uncomment(v string) string {
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// SetFields writes top-level frontmatter fields: a field that exists is
// replaced on its line, one that does not goes before the closing fence. Every
// other line, comments included, stays as it was, and so do line endings.
func SetFields(doc string, fields [][2]string) (string, error) {
	eol := "\n"
	if strings.Contains(doc, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("sem frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", fmt.Errorf("frontmatter sem a linha --- de fechamento")
	}
	for _, f := range fields {
		line := f[0] + ": " + f[1]
		found := false
		for i := 1; i < end; i++ {
			if strings.HasPrefix(lines[i], f[0]+":") {
				lines[i], found = line, true
				break
			}
		}
		if !found {
			lines = append(lines[:end], append([]string{line}, lines[end:]...)...)
			end++
		}
	}
	return strings.Join(lines, eol), nil
}
