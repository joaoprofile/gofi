package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// InstallGo installs the Go scaffold at projectRoot — go.work plus the source
// folder (data.SourceRoot, e.g. "src" or "services") containing go.mod,
// main.go, domain/ and .migrations/. The directory must already exist.
//
// data.SourceRoot is required; the installer replaces the RootMarker in path
// components and references {{.SourceRoot}} from go.work.tmpl.
func InstallGo(projectRoot string, data TemplateData) ([]string, error) {
	return installEmbedded("golang", projectRoot, data)
}

// EnsureGoWorkSDK keeps <projectRoot>/go.work aligned with the local SDK
// checkout at .gofi/gofi-sdk-<language>/. Every directory inside the checkout
// that contains a go.mod becomes a `use` entry — multi-module SDKs (e.g. the
// gofi-sdk-go repo, where each capability has its own module) need every
// submodule wired in for the toolchain to resolve their imports, since Go
// workspaces don't traverse nested go.mod files transitively.
//
// The function also bumps the leading `go X.Y.Z` directive in go.work to the
// max of its current value and the version declared by each SDK go.mod —
// without it the toolchain refuses the workspace with "module X requires
// go >= 1.Y.Z, but go.work lists go 1.Y" when the workspace declares a less
// specific version than the modules require.
//
// The function fully owns the use entries under .gofi/gofi-sdk-<language>/:
// previously managed paths are dropped first, then the freshly discovered set
// is appended. Other use entries (e.g. ./src) and unrelated content (header,
// replace directives, comments) are preserved verbatim. The use directive is
// emitted as the grouped `use ( ... )` form when there are multiple paths,
// single-line otherwise. No-op when go.work doesn't exist.
func EnsureGoWorkSDK(projectRoot, language string) error {
	workPath := filepath.Join(projectRoot, "go.work")
	raw, err := os.ReadFile(workPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	managedPrefix := "./.gofi/gofi-sdk-" + language
	modules, err := collectSDKModules(projectRoot, language)
	if err != nil {
		return err
	}

	desired := make([]string, len(modules))
	requiredGo := parseGoDirective(string(raw))
	for i, m := range modules {
		desired[i] = m.RelPath
		if cmpGoVersions(m.GoVersion, requiredGo) > 0 {
			requiredGo = m.GoVersion
		}
	}
	sort.Strings(desired)

	updated, changed := rewriteGoWorkUses(string(raw), managedPrefix, desired)
	if requiredGo != "" {
		bumped := setGoDirective(updated, requiredGo)
		if bumped != updated {
			updated = bumped
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return os.WriteFile(workPath, []byte(updated), 0o644)
}

// sdkModule describes one Go module inside the SDK checkout.
type sdkModule struct {
	RelPath   string // path relative to projectRoot, with leading "./"
	GoVersion string // version declared by `go X.Y[.Z]` in go.mod, may be empty
}

// collectSDKModules walks .gofi/gofi-sdk-<language>/ and returns one entry per
// directory containing a go.mod, sorted by RelPath. Returns nil (and no
// error) when the SDK checkout is absent. RelPath uses forward slashes and a
// leading "./" so it can be inserted directly into go.work.
func collectSDKModules(projectRoot, language string) ([]sdkModule, error) {
	base := filepath.Join(projectRoot, ".gofi", "gofi-sdk-"+language)
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	var modules []sdkModule
	walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Name() != "go.mod" {
			return nil
		}
		modDir := filepath.Dir(p)
		rel, relErr := filepath.Rel(projectRoot, modDir)
		if relErr != nil {
			return relErr
		}
		body, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		modules = append(modules, sdkModule{
			RelPath:   "./" + filepath.ToSlash(rel),
			GoVersion: parseGoDirective(string(body)),
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].RelPath < modules[j].RelPath })
	return modules, nil
}

// goDirectivePattern matches lines like "go 1.25", "go 1.25.0", possibly with
// trailing whitespace or comments. Used both in go.mod and go.work.
var goDirectivePattern = regexp.MustCompile(`^\s*go\s+([0-9]+(?:\.[0-9]+){1,2})\b`)

// parseGoDirective returns the version string from the first `go X.Y[.Z]`
// directive found in content, or "" if none.
func parseGoDirective(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if m := goDirectivePattern.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

// setGoDirective rewrites raw so its first `go X.Y[.Z]` directive becomes
// `go <version>`. Returns raw unchanged when the parsed value already equals
// version, when version is empty, or when no directive is present (we never
// inject one — go.work is expected to already declare it).
func setGoDirective(raw, version string) string {
	if version == "" {
		return raw
	}
	current := parseGoDirective(raw)
	if current == version {
		return raw
	}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if goDirectivePattern.MatchString(line) {
			lines[i] = "go " + version
			return strings.Join(lines, "\n")
		}
	}
	return raw
}

// cmpGoVersions compares two go-directive versions like "1.25" and "1.25.0".
// When numerically equal but with different precision the more precise one
// wins — the Go toolchain treats `go 1.25.0` as a stricter requirement than
// `go 1.25` when validating workspace vs module directives, so a module
// declaring `1.25.0` must bump a workspace declaring `1.25`. Returns -1 if
// a < b, 0 if equal, +1 if a > b. Empty strings sort below everything.
func cmpGoVersions(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	pa := splitGoVersion(a)
	pb := splitGoVersion(b)
	for i := 0; i < 3; i++ {
		ai, bi := 0, 0
		if i < len(pa) {
			ai = pa[i]
		}
		if i < len(pb) {
			bi = pb[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	switch {
	case len(pa) > len(pb):
		return 1
	case len(pa) < len(pb):
		return -1
	default:
		return 0
	}
}

// splitGoVersion parses "1.25" or "1.25.0" into a slice of ints, preserving
// precision (no zero-padding) so cmpGoVersions can use length to break ties.
func splitGoVersion(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}

// rewriteGoWorkUses returns the new go.work content with every use path
// matching managedPrefix replaced by the desired list, plus a flag indicating
// whether the content changed. Accepts both `use ./path` and `use ( ... )`
// forms and emits the canonical form for the resulting path count.
//
// "Matching managedPrefix" means the path equals managedPrefix or starts with
// managedPrefix+"/" — so a desired list of submodules at managedPrefix/<sub>
// fully replaces any prior entries (including the bare prefix itself if a
// previous run added it before submodule discovery existed).
func rewriteGoWorkUses(raw, managedPrefix string, desired []string) (string, bool) {
	lines := strings.Split(raw, "\n")
	var (
		before  []string
		after   []string
		paths   []string
		seenUse bool
		inBlock bool
	)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if !seenUse {
			if strings.HasPrefix(trim, "use (") {
				seenUse = true
				inBlock = true
				continue
			}
			if strings.HasPrefix(trim, "use ") {
				seenUse = true
				paths = append(paths, strings.TrimSpace(strings.TrimPrefix(trim, "use ")))
				continue
			}
			before = append(before, line)
			continue
		}
		if inBlock {
			if trim == ")" {
				inBlock = false
				continue
			}
			if trim == "" || strings.HasPrefix(trim, "//") {
				continue
			}
			paths = append(paths, trim)
			continue
		}
		after = append(after, line)
	}
	if !seenUse {
		return raw, false
	}

	kept := paths[:0]
	for _, p := range paths {
		if p == managedPrefix || strings.HasPrefix(p, managedPrefix+"/") {
			continue
		}
		kept = append(kept, p)
	}
	kept = append(kept, desired...)

	seen := map[string]bool{}
	final := kept[:0]
	for _, p := range kept {
		if seen[p] {
			continue
		}
		seen[p] = true
		final = append(final, p)
	}

	var rebuilt strings.Builder
	rebuilt.WriteString(strings.Join(before, "\n"))
	if len(before) > 0 {
		rebuilt.WriteString("\n")
	}
	switch len(final) {
	case 0:
		// no use paths left — drop the directive entirely
	case 1:
		rebuilt.WriteString("use ")
		rebuilt.WriteString(final[0])
		rebuilt.WriteString("\n")
	default:
		rebuilt.WriteString("use (\n")
		for _, p := range final {
			rebuilt.WriteString("\t")
			rebuilt.WriteString(p)
			rebuilt.WriteString("\n")
		}
		rebuilt.WriteString(")\n")
	}
	if len(after) > 0 {
		rebuilt.WriteString(strings.Join(after, "\n"))
	}

	out := rebuilt.String()
	return out, out != raw
}
