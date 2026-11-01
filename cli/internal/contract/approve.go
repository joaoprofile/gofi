package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// The fields an approval writes into the spec's frontmatter. They sit next to
// status, in git, reviewed like the rest of the spec: an approval is the
// team's state, not a local one. No name and no date — provenance lives in git.
const (
	FieldSeal        = "aprovacao_sha"
	FieldSealVersion = "aprovacao_versao"
	StatusApproved   = "aprovado"
)

// Seal is the hash of what the person approved: the body, without the
// frontmatter, so writing the approval does not change what it seals. Line
// endings and trailing blank lines do not count; every other byte does.
func Seal(body []string) string {
	text := strings.TrimRight(strings.Join(body, "\n"), " \t\r\n")
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SealState is where a spec stands with its approval.
type SealState int

const (
	Unsealed SealState = iota // never approved through gofi
	Sealed                    // approved, and unchanged since
	Stale                     // approved, then edited: the approval no longer holds
)

// SealOf reads the approval a spec carries against what it says now. The
// reason says why a seal is stale.
func SealOf(s Spec) (state SealState, reason string) {
	seal := value(s.Front, FieldSeal)
	if seal == "" {
		return Unsealed, ""
	}
	if seal != Seal(s.Body) {
		return Stale, "o corpo mudou depois da aprovação"
	}
	if v := value(s.Front, FieldSealVersion); v != value(s.Front, "versao") {
		return Stale, fmt.Sprintf("aprovada na versão %s, a spec está na %s", v, value(s.Front, "versao"))
	}
	return Sealed, ""
}

// sealFindings are the approval's own findings: a stale seal blocks the loop,
// an approved status no seal backs is worth knowing.
func sealFindings(s Spec) []docs.Finding {
	switch state, reason := SealOf(s); {
	case state == Stale:
		return []docs.Finding{{Path: s.Path, Error: true,
			Message: "aprovação vencida: " + reason + " — revise e rode `gofi spec approve " + s.Context + "`"}}
	case state == Unsealed && value(s.Front, "status") == StatusApproved:
		return []docs.Finding{{Path: s.Path,
			Message: "status aprovado sem selo — rode `gofi spec approve " + s.Context + "` para o loop poder implementá-la"}}
	}
	return nil
}

// StaleSeals reports the specs whose approval no longer holds, for the corpus
// check. An approved spec with no seal is left out there: that is every spec
// written before approvals were sealed, and the check would be noise.
func StaleSeals(root string) []docs.Finding {
	specs, err := Specs(root)
	if err != nil {
		return nil
	}
	var out []docs.Finding
	for _, s := range specs {
		if state, _ := SealOf(s); state == Stale {
			out = append(out, sealFindings(s)...)
		}
	}
	return out
}

// Blocking is what keeps a spec from being approved, out of the whole corpus
// (all): the content, dependency and corpus findings that are errors. A stale
// seal is not among them — approving again is how a stale seal is cleared.
func Blocking(root string, all []Spec, s Spec, placeholders map[string]bool) []docs.Finding {
	var out []docs.Finding
	found := append(content(s, placeholders), depFindings(all, map[string]bool{s.Path: true})...)
	for _, f := range found {
		if f.Error {
			out = append(out, f)
		}
	}
	for _, f := range docs.Validate(root) {
		if f.Path == s.Path && f.Error {
			out = append(out, f)
		}
	}
	return out
}

// Approve seals a spec: status aprovado, the version approved and the hash of
// its body, written into its frontmatter in place. It reports false when the
// spec was already sealed as it is, and writes nothing then.
func Approve(root string, s Spec) (bool, error) {
	if state, _ := SealOf(s); state == Sealed && value(s.Front, "status") == StatusApproved {
		return false, nil
	}
	path := filepath.Join(root, filepath.FromSlash(s.Path))
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	out, err := setFields(string(raw), [][2]string{
		{"status", StatusApproved},
		{FieldSealVersion, `"` + value(s.Front, "versao") + `"`},
		{FieldSeal, `"` + Seal(s.Body) + `"`},
	})
	if err != nil {
		return false, fmt.Errorf("%s: %w", s.Path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}

// setFields writes top-level frontmatter fields: a field that exists is
// replaced on its line, one that does not goes before the closing fence. Every
// other line, comments included, stays as it was, and so do line endings.
func setFields(doc string, fields [][2]string) (string, error) {
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
