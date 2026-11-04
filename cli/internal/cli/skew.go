package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

// EnvAllowSkew lets a binary work with skills installed from another commit
// of gofi, on purpose.
const EnvAllowSkew = "GOFI_ALLOW_SKEW"

// binaryCommit is the commit this binary was built from, "" when the build
// does not say (go run, tests).
func binaryCommit() string {
	c := strings.TrimSuffix(strings.TrimSpace(Commit), "-dirty")
	if c == "" || c == "none" || c == "unknown" {
		return ""
	}
	return c
}

// sameCommit compares two commit ids, full or abbreviated.
func sameCommit(a, b string) bool {
	n := min(len(a), len(b))
	return n >= 7 && strings.EqualFold(a[:n], b[:n])
}

// skewAllowed reports whether a commit of skills other than the binary's may
// be used: asked for, unknown on either side, or installed from a local folder.
func skewAllowed(agents string) bool {
	return os.Getenv(EnvAllowSkew) != "" || binaryCommit() == "" || agents == "" || agents == "local"
}

// checkAgentsSkew refuses to work with the skills installed in a project when
// they come from another commit of gofi than this binary: the skills and the
// binary name the same folders, files and commands only when they are of the
// same commit, and one that differs writes where the other no longer reads.
func checkAgentsSkew(projectRoot string) error {
	agents := readInstalledSha(projectRoot)
	if skewAllowed(agents) || sameCommit(agents, binaryCommit()) {
		return nil
	}
	return errors.New(i18n.T("skew.installed", short(agents), binaryCommit(), EnvAllowSkew))
}

// checkSourceSkew refuses to install gofi content resolved to another commit
// than this binary's.
func checkSourceSkew(resolved string) error {
	if skewAllowed(resolved) || sameCommit(resolved, binaryCommit()) {
		return nil
	}
	return errors.New(i18n.T("skew.source", short(resolved), binaryCommit(), EnvAllowSkew))
}
