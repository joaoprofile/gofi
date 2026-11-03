package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// commitFile is where a run keeps the commit it started from, beside the
// graph it started from.
func commitFile(work, run string) string {
	return filepath.Join(work, filepath.FromSlash(loop.BaselineDir(run)), "commit")
}

// protectedFile is where a run keeps what its protected files were when it
// started: the open change it carries out is there before any commit.
func protectedFile(work, run string) string {
	return filepath.Join(work, filepath.FromSlash(loop.BaselineDir(run)), "protected.json")
}

// captureIntegrity keeps what a run starts from, for the integrity stage to
// tell what the run changed: the commit, and the hash of every protected
// file. A resumed run keeps the one it started from.
func captureIntegrity(ctx context.Context, work, run string) error {
	if _, err := os.Stat(protectedFile(work, run)); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(commitFile(work, run)), 0o755); err != nil {
		return err
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", work, "rev-parse", "HEAD").Output(); err == nil {
		if err := os.WriteFile(commitFile(work, run), bytes.TrimSpace(out), 0o644); err != nil {
			return err
		}
	}
	data, err := json.Marshal(protectedHashes(work))
	if err != nil {
		return err
	}
	return os.WriteFile(protectedFile(work, run), data, 0o644)
}

// protectedHashes are the protected files of a tree and their hashes.
func protectedHashes(root string) map[string]string {
	out := map[string]string{}
	for _, top := range []string{"specs", "prd", layout.Home()} {
		_ = filepath.WalkDir(filepath.Join(root, top), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if !protected(rel) {
				return nil
			}
			if b, err := os.ReadFile(p); err == nil {
				sum := sha256.Sum256(b)
				out[rel] = hex.EncodeToString(sum[:])
			}
			return nil
		})
	}
	return out
}

// integritySensor is gofi's own stage (0002, D12, S6): the run cannot win by
// cheating — editing its contract, deleting a test, skipping one.
func integritySensor(run string) pack.Command {
	if runPorts.integrity != nil {
		return *runPorts.integrity
	}
	exe, err := os.Executable()
	if err != nil {
		return pack.Command{}
	}
	report := filepath.ToSlash(filepath.Join(loop.RunsDir, run, "integrity.sarif"))
	return pack.Command{Run: fmt.Sprintf("%q integrity --run %s --report %s", exe, run, report), Format: pack.FormatSARIF, Report: report}
}

func newIntegrityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "integrity",
		Short:  "Check that a run did not win by cheating (the loop's integrity stage)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			run, _ := cmd.Flags().GetString("run")
			report, _ := cmd.Flags().GetString("report")
			lp, err := projectPack(cfg)
			if err != nil {
				return err
			}
			found, err := integrity(cmd.Context(), root, run, lp)
			if err != nil {
				return err
			}
			if report != "" {
				if err := writeSARIF(filepath.Join(root, filepath.FromSlash(report)), "gofi-integrity", found); err != nil {
					return err
				}
			}
			for _, f := range found {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s:%d %s\n", f.URI, f.Line, f.Message)
			}
			if len(found) > 0 {
				return &ExitCodeError{Code: 1, Err: fmt.Errorf("%d violações de integridade", len(found))}
			}
			return nil
		},
	}
	cmd.Flags().String("run", "", "the run whose changes are checked")
	cmd.Flags().String("report", "", "where to write the violations, as SARIF")
	return cmd
}

// integrity is what a run changed that it may not (0002, D12): a file of
// the contract or of the harness — specs/, prd/, the agents' folder but the
// memory gofi writes —, a test removed, a skip added to a test. Each is a
// blocker: the work is refused, not fixed around.
func integrity(ctx context.Context, root, run string, lp *pack.Pack) ([]sarifResult, error) {
	var out []sarifResult
	if data, err := os.ReadFile(protectedFile(root, run)); err == nil {
		var was map[string]string
		if json.Unmarshal(data, &was) == nil {
			now := protectedHashes(root)
			var paths []string
			for p, h := range now {
				if was[p] != h {
					paths = append(paths, p)
				}
			}
			for p := range was {
				if _, ok := now[p]; !ok {
					paths = append(paths, p)
				}
			}
			slices.Sort(paths)
			for _, p := range paths {
				out = append(out, sarifResult{Rule: "integrity: contrato", URI: p,
					Message: "a execução alterou " + p + ", que não é dela: o contrato e o harness mudam só pela pessoa — desfaça a alteração"})
			}
		}
	}
	since, err := os.ReadFile(commitFile(root, run))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(string(since))
	git := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...).Output()
		return string(out), err
	}
	status, err := git("diff", "--name-status", "--no-renames", base, "--", ".")
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	untracked, _ := git("ls-files", "--others", "--exclude-standard", "--", ".")
	type change struct{ op, path string }
	var changes []change
	for _, ln := range strings.Split(strings.TrimSpace(status), "\n") {
		if op, p, ok := strings.Cut(ln, "\t"); ok {
			changes = append(changes, change{op, filepath.ToSlash(p)})
		}
	}
	for _, p := range strings.Fields(untracked) {
		changes = append(changes, change{"A", filepath.ToSlash(p)})
	}
	for _, c := range changes {
		switch {
		case protected(c.path):
			// Judged by the snapshot above: the open change is there before
			// any commit.
		case lp != nil && lp.IsTest(c.path) && c.op == "D":
			out = append(out, sarifResult{Rule: "integrity: teste removido", URI: c.path,
				Message: "a execução removeu o teste " + c.path + ": um teste que falha se corrige pelo código, não se apaga — restaure-o"})
		case lp != nil && lp.IsTest(c.path):
			for _, at := range skipsAdded(git, base, root, c.path, c.op, lp.Skips) {
				out = append(out, sarifResult{Rule: "integrity: teste pulado", URI: c.path, Line: at,
					Message: "a execução marcou um teste para ser pulado em " + c.path + ": um teste pulado não prova nada — faça-o passar"})
			}
		}
	}
	return out, nil
}

// protected is a path a run may not change: the contract and the harness.
// The memory, the derived indexes and gofi's own folder are gofi's to write.
func protected(p string) bool {
	if derivedFile(p) || strings.HasPrefix(p, ".gofi/") {
		return false
	}
	if strings.HasPrefix(p, layout.Memory().Dir+"/") {
		return false
	}
	return strings.HasPrefix(p, "specs/") || strings.HasPrefix(p, "prd/") || strings.HasPrefix(p, layout.Home()+"/")
}

// skipsAdded are the lines of a test file where the run added a skip marker
// of the language: every line of a new file, the added ones of an edited one.
func skipsAdded(git func(...string) (string, error), base, root, file, op string, skips []string) []int {
	if len(skips) == 0 {
		return nil
	}
	has := func(ln string) bool {
		return slices.ContainsFunc(skips, func(m string) bool { return strings.Contains(ln, m) })
	}
	var at []int
	if op == "A" {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return nil
		}
		for i, ln := range strings.Split(string(data), "\n") {
			if has(ln) {
				at = append(at, i+1)
			}
		}
		return at
	}
	diff, err := git("diff", "-U0", base, "--", file)
	if err != nil {
		return nil
	}
	line := 0
	sc := bufio.NewScanner(strings.NewReader(diff))
	for sc.Scan() {
		ln := sc.Text()
		if strings.HasPrefix(ln, "@@") {
			// @@ -a,b +c,d @@: the new lines start at c.
			if _, after, ok := strings.Cut(ln, "+"); ok {
				fmt.Sscanf(after, "%d", &line)
			}
			continue
		}
		if strings.HasPrefix(ln, "+") && !strings.HasPrefix(ln, "+++") {
			if has(ln) {
				at = append(at, line)
			}
			line++
		}
	}
	return at
}
