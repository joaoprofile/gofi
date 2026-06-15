package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/joaoprofile/gofi-cli/internal/scaffold"
	"github.com/joaoprofile/gofi-cli/internal/sources"
)

// fetchSource resolves and downloads a generic github.com/<org>/<repo>@<ref>
// to the project-local cache (<projectRoot>/.gofi/cache by default, or
// $GOFI_CACHE_DIR when the env var is set). Returns the extracted directory
// and the resolved Ref.
//
// $GOFI_AGENTS_LOCAL_DIR short-circuits the fetch and uses a local directory
// directly — used in tests to avoid hitting GitHub.
func fetchSource(projectRoot, ref string) (string, sources.Ref, error) {
	if local := os.Getenv("GOFI_AGENTS_LOCAL_DIR"); local != "" {
		return local, sources.Ref{Ref: "local"}, nil
	}
	r, err := sources.Parse(ref)
	if err != nil {
		return "", sources.Ref{}, err
	}
	cache, err := sources.ProjectCache(projectRoot)
	if err != nil {
		return "", sources.Ref{}, err
	}
	client, err := sources.NewClient(cache)
	if err != nil {
		return "", sources.Ref{}, err
	}
	resolved, err := client.Resolve(r)
	if err != nil {
		return "", sources.Ref{}, err
	}
	dir, err := client.FetchTarball(resolved)
	if err != nil {
		return "", sources.Ref{}, err
	}
	return dir, resolved, nil
}

// fetchSDKToProject ensures <projectRoot>/.gofi/gofi-sdk-<language>/ holds the
// extracted contents of sdkRef at the resolved SHA, returning that path. The
// directory is the live checkout consumed by the toolchain (referenced from
// go.work, importable from the project's Go module) — distinct from the
// .gofi/cache used internally for gofi-agents.
//
// Returns ("", Ref{}, nil) when sdkRef is empty or when test fixtures are
// active without an SDK-specific override (so .gofi/gofi-sdk-<lang>/ doesn't
// get polluted by unrelated agents fixture content).
//
// $GOFI_SDK_LOCAL_DIR short-circuits the fetch and mirrors a local directory
// into the destination — used in tests that exercise the SDK override path
// without hitting GitHub. When unset and $GOFI_AGENTS_LOCAL_DIR is set
// (agents-only fixture), the SDK fetch is skipped entirely.
func fetchSDKToProject(projectRoot, language, sdkRef string) (string, sources.Ref, error) {
	if sdkRef == "" || language == "" {
		return "", sources.Ref{}, nil
	}
	dest := filepath.Join(projectRoot, ".gofi", "gofi-sdk-"+language)

	if local := os.Getenv("GOFI_SDK_LOCAL_DIR"); local != "" {
		if err := mirrorDir(local, dest); err != nil {
			return "", sources.Ref{}, fmt.Errorf("mirror SDK fixture: %w", err)
		}
		return dest, sources.Ref{Ref: "local"}, nil
	}
	if os.Getenv("GOFI_AGENTS_LOCAL_DIR") != "" {
		return "", sources.Ref{}, nil
	}

	r, err := sources.Parse(sdkRef)
	if err != nil {
		return "", sources.Ref{}, err
	}
	cache, err := sources.ProjectCache(projectRoot)
	if err != nil {
		return "", sources.Ref{}, err
	}
	client, err := sources.NewClient(cache)
	if err != nil {
		return "", sources.Ref{}, err
	}
	resolved, err := client.Resolve(r)
	if err != nil {
		return "", sources.Ref{}, err
	}
	if storedSHA := readInstalledSDKSha(projectRoot, language); storedSHA == resolved.Ref {
		if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
			return dest, resolved, nil
		}
	}
	if err := client.FetchTarballTo(resolved, dest); err != nil {
		return "", sources.Ref{}, err
	}
	return dest, resolved, nil
}

// installFromSource downloads the gofi-agents repo and installs its content
// (agents, templates, memory) into <projectRoot>/.claude/. When sdkRef is
// configured, the SDK is also extracted to <projectRoot>/.gofi/gofi-sdk-<lang>/
// (live checkout for the toolchain) and its docs feed .claude/sdk/<lang>/.
// When the override doesn't expose a docs layout (boilerplates/, sdk-docs/,
// knowledge/), docs fall back to gofi-agents/sdk/<lang>/ — the .gofi checkout
// stays in place either way so go.work / Go imports keep working.
//
// There is no embedded fallback — a network or repo error propagates.
//
// Returns the resolved SHA of the agents repo on success.
func installFromSource(projectRoot, language string, uiSurfaces []string, agentsRef, sdkRef string, data scaffold.TemplateData, mode scaffold.InstallMode) (string, error) {
	dir, resolved, err := fetchSource(projectRoot, agentsRef)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", agentsRef, err)
	}
	fsys := os.DirFS(dir)
	if !dirExists(fsys, "ai/skills") {
		return "", errors.New("repo does not contain ai/skills/ — verify the URL is a gofi monorepo")
	}

	if _, err := scaffold.InstallAgentsContent(fsys, ".", projectRoot, data, mode); err != nil {
		return "", fmt.Errorf("install agents content: %w", err)
	}

	// Front-end design-system docs (gofi-ui / gofi-ui-native) for surfaces that
	// use a DS. Installed from the monorepo tree; harmless for front-only.
	for _, surface := range uiSurfaces {
		if _, err := scaffold.InstallUIContent(fsys, "ai/sdk/"+surface, projectRoot, surface); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not install %s design-system docs: %v\n", surface, err)
		}
	}

	if language == "" {
		return resolved.Ref, nil
	}

	docsInstalled := false
	if sdkRef != "" {
		ok, err := tryInstallSDKOverride(projectRoot, language, sdkRef)
		if err != nil {
			return "", err
		}
		docsInstalled = ok
	}
	if !docsInstalled {
		if _, err := scaffold.InstallSDKContent(fsys, "ai/sdk/"+language, projectRoot, language); err != nil {
			if errors.Is(err, scaffold.ErrNoSDKLayout) {
				fmt.Fprintf(os.Stderr, "warning: gofi/ai/sdk/%s/ has no SDK layout (boilerplates/, sdk-docs/, knowledge/); skipping docs install\n", language)
				return resolved.Ref, nil
			}
			return "", fmt.Errorf("install sdk content: %w", err)
		}
	}
	return resolved.Ref, nil
}

// tryInstallSDKOverride extracts sdkRef into .gofi/gofi-sdk-<language>/ and,
// when that checkout exposes a docs layout, copies the curated subset into
// .claude/sdk/<language>/. Returns (true, nil) when docs were installed from
// the override, (false, nil) when the caller should fall back to the bundled
// gofi-agents docs (override unreachable or missing the docs layout), and
// (false, err) on a hard failure that should abort the pipeline. The .gofi/
// checkout itself is preserved on the docs-missing fallback so the toolchain
// can still consume the override as a Go module.
func tryInstallSDKOverride(projectRoot, language, sdkRef string) (bool, error) {
	sdkDir, resolved, err := fetchSDKToProject(projectRoot, language, sdkRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: SDK override fetch failed (%v); falling back to gofi-agents/sdk/%s/\n", err, language)
		return false, nil
	}
	if sdkDir == "" {
		return false, nil
	}
	sdkFS := os.DirFS(sdkDir)
	if _, err := scaffold.InstallSDKContent(sdkFS, ".", projectRoot, language); err != nil {
		if errors.Is(err, scaffold.ErrNoSDKLayout) {
			fmt.Fprintf(os.Stderr, "warning: SDK override %s does not contain a gofi SDK layout (boilerplates/, sdk-docs/, knowledge/); falling back to gofi-agents/sdk/%s/\n", sdkRef, language)
			recordSDKSha(projectRoot, language, resolved)
			return false, nil
		}
		return false, fmt.Errorf("install sdk content (override): %w", err)
	}
	recordSDKSha(projectRoot, language, resolved)
	return true, nil
}

// recordSDKSha persists the resolved SHA into installed.yaml so subsequent
// invocations can skip re-extracting the same ref. The "local" sentinel from
// fixture-driven runs is intentionally not recorded.
func recordSDKSha(projectRoot, language string, resolved sources.Ref) {
	if resolved.Ref == "" || resolved.Ref == "local" {
		return
	}
	if err := writeInstalledSDKSha(projectRoot, language, resolved.Ref); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record SDK SHA: %v\n", err)
	}
}

func dirExists(fsys fs.FS, p string) bool {
	info, err := fs.Stat(fsys, p)
	return err == nil && info.IsDir()
}

// mirrorDir replaces dst with a deep copy of src. Used for fixture-driven
// tests where an env var points at a local SDK source instead of GitHub —
// keeping the rest of the install flow working against a real on-disk path.
func mirrorDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
