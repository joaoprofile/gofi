package docs

import (
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

// List reads a [a, b, c] field. Missing or empty yields nil.
func (f Frontmatter) List(key string) []string {
	v := strings.TrimSpace(f[key])
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
// uses it to mean "none".
func (f Frontmatter) Get(key string) string {
	v := strings.Trim(strings.TrimSpace(f[key]), `"`)
	if v == "n/a" {
		return ""
	}
	return v
}
