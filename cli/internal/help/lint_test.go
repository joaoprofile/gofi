package help

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestLint_AllFilled(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "r"}
	child := &cobra.Command{
		Use:     "child",
		Short:   "child short",
		Long:    "child long",
		Example: "root child",
	}
	root.AddCommand(child)
	if errs := Lint(root); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestLint_MissingFields(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "r"}
	root.AddCommand(&cobra.Command{Use: "no-short"})
	root.AddCommand(&cobra.Command{Use: "no-long", Short: "x", Example: "root no-long"})
	root.AddCommand(&cobra.Command{Use: "no-example", Short: "x", Long: "y"})
	errs := Lint(root)
	if len(errs) < 4 {
		t.Fatalf("expected at least 4 errors, got %d: %v", len(errs), errs)
	}
	joined := joinErrs(errs)
	for _, must := range []string{"no-short: Short is empty", "no-short: Long is empty", "no-long: Long is empty", "no-example: Example is empty"} {
		if !strings.Contains(joined, must) {
			t.Errorf("expected error containing %q in: %s", must, joined)
		}
	}
}

func TestLint_SkipsHelpCommand(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "r"}
	root.AddCommand(&cobra.Command{Use: "help"})
	if errs := Lint(root); len(errs) != 0 {
		t.Fatalf("expected no errors for help cmd, got: %v", errs)
	}
}

func joinErrs(errs []error) string {
	var s []string
	for _, e := range errs {
		s = append(s, e.Error())
	}
	return strings.Join(s, "\n")
}
