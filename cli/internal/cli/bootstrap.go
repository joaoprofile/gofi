package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/settings"
	"github.com/joaoprofile/gofi-cli/internal/tui/styles"
	"github.com/joaoprofile/gofi-cli/internal/tui/wizard"
)

// bootstrapSettings loads gofi.json before the command tree is built, so every
// description already renders in the user's language.
//
// On the first run it offers the setup wizard and persists the answers. When
// the session cannot ask (no terminal, GOFI_NO_SETUP, shell completion) it runs
// on the detected defaults and writes nothing, so CI never blocks on a prompt.
func bootstrapSettings() {
	s, err := settings.Load()
	if err == nil {
		settings.Use(s)
		return
	}
	if !errors.Is(err, settings.ErrNotConfigured) {
		// a corrupt file must not stop the CLI: warn and run on defaults
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		settings.Use(settings.Default())
		return
	}

	def := settings.Default()
	settings.Use(def)
	if !canPromptForSetup() {
		return
	}

	out, err := wizard.RunSetup(def)
	if err != nil {
		if !errors.Is(err, wizard.ErrSetupCancelled) {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		fmt.Println(styles.Note(i18n.T("setup.cancelled")))
		settings.Use(def)
		return
	}

	settings.Use(out)
	if err := settings.Save(out); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("setup.save_failed", err))
		return
	}
	fmt.Println()
	fmt.Println(styles.Success(i18n.T("setup.saved", out.Path())))
	fmt.Println(styles.Note(i18n.T("setup.next")))
	fmt.Println()
}

// canPromptForSetup reports whether this invocation may show the wizard.
func canPromptForSetup() bool {
	if os.Getenv(settings.EnvNoSetup) != "" {
		return false
	}
	if isCompletionRequest() {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// isCompletionRequest detects cobra completion, which expects candidates and
// an exit, never a form.
func isCompletionRequest() bool {
	if len(os.Args) < 2 {
		return false
	}
	arg := os.Args[1]
	return arg == "completion" || strings.HasPrefix(arg, "__complete")
}

// interactiveTerminal reports whether we can run a form right now.
func interactiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// languageLabel renders a code as endonym plus code, e.g. "Português (pt)".
func languageLabel(code string) string {
	if l, ok := i18n.Lookup(code); ok {
		return fmt.Sprintf("%s (%s)", l.Native, l.Code)
	}
	return code
}

// settingsKeyList is the key list used in error messages.
func settingsKeyList() string { return strings.Join(settings.Keys(), ", ") }

// settingsValueList is the accepted-value list for a key.
func settingsValueList(key string) string {
	vals := settings.ValidValues(key)
	if len(vals) == 0 {
		return ""
	}
	return strings.Join(slices.Clone(vals), " | ")
}
