package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// ExtractorsDir is where `gofi graph install` puts extractors, relative to the
// project root. Keeping them inside the project rather than in a shared
// location means two projects can pin different extractor versions.
const ExtractorsDir = ".gofi/graph/extractors"

// BinaryPrefix is the naming convention an extractor must follow to be found.
const BinaryPrefix = "gofi-graph-"

// BinaryName is the executable name for a language, with the platform suffix.
func BinaryName(language string) string {
	name := BinaryPrefix + strings.ToLower(language)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Find locates the extractor for a language. The project's own directory wins
// over PATH, so a pinned extractor is never shadowed by a stray global install.
func Find(projectRoot, language string) (Spec, error) {
	name := BinaryName(language)

	local := filepath.Join(projectRoot, ExtractorsDir, name)
	if isExecutable(local) {
		return Spec{Language: language, Path: local}, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return Spec{Language: language, Path: p}, nil
	}
	return Spec{}, fmt.Errorf(
		"nenhum extractor para %q — instale com `gofi graph install %s`, "+
			"ou ponha um executavel %s no PATH", language, language, name)
}

// Installed lists the languages that have an extractor available to this
// project, in a stable order.
func Installed(projectRoot string) []string {
	seen := map[string]bool{}

	entries, _ := os.ReadDir(filepath.Join(projectRoot, ExtractorsDir))
	for _, e := range entries {
		if lang := languageOf(e.Name()); lang != "" && !e.IsDir() {
			seen[lang] = true
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if lang := languageOf(e.Name()); lang != "" {
				seen[lang] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for lang := range seen {
		out = append(out, lang)
	}
	slices.Sort(out)
	return out
}

// languageOf extracts the language from an extractor file name, or "" when the
// name does not follow the convention.
func languageOf(fileName string) string {
	name := strings.TrimSuffix(fileName, ".exe")
	if !strings.HasPrefix(name, BinaryPrefix) {
		return ""
	}
	return strings.TrimPrefix(name, BinaryPrefix)
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	// Windows decides by extension, not by a permission bit.
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}
