package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/settings"
	"github.com/joaoprofile/gofi-cli/internal/tui/styles"
	"github.com/joaoprofile/gofi-cli/internal/tui/wizard"
)

// newSettingsCmd builds `gofi settings`: the CLI own configuration, while
// `gofi config` edits the current project .gofi.yaml.
func newSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "settings",
		Aliases: []string{"prefs"},
		Short:   i18n.T("cmd.settings.short"),
		Long:    i18n.T("cmd.settings.long"),
		Example: `gofi settings
gofi settings set language pt
gofi settings set output plain
gofi settings wizard
gofi settings path`,
		Annotations: map[string]string{
			"related": i18n.T("cmd.settings.related"),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSettingsShow()
		},
	}
	cmd.AddCommand(
		newSettingsShowCmd(),
		newSettingsSetCmd(),
		newSettingsWizardCmd(),
		newSettingsPathCmd(),
		newSettingsResetCmd(),
	)
	return cmd
}

func newSettingsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Aliases: []string{"list", "ls"},
		Short:   i18n.T("cmd.settings.show.short"),
		Long:    i18n.T("cmd.settings.show.long"),
		Example: `gofi settings show
gofi settings show --plain`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSettingsShow()
		},
	}
}

func newSettingsSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: i18n.T("cmd.settings.set.short"),
		Long:  i18n.T("cmd.settings.set.long"),
		Example: `gofi settings set language pt
gofi settings set language=fr
gofi settings set color never
gofi settings set output plain
gofi settings set checkin false`,
		Args: cobra.RangeArgs(1, 2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return settings.Keys(), cobra.ShellCompDirectiveNoFileComp
			}
			return settings.ValidValues(args[0]), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value, err := parseSettingArgs(args)
			if err != nil {
				return err
			}
			return runSettingsSet(key, value)
		},
	}
}

func newSettingsWizardCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "wizard",
		Aliases: []string{"setup"},
		Short:   i18n.T("cmd.settings.wizard.short"),
		Long:    i18n.T("cmd.settings.wizard.long"),
		Example: `gofi settings wizard`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSettingsWizard()
		},
	}
}

func newSettingsPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "path",
		Short:   i18n.T("cmd.settings.path.short"),
		Long:    i18n.T("cmd.settings.path.long"),
		Example: `gofi settings path`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, exists, err := settings.Locate()
			if err != nil {
				return err
			}
			fmt.Println(path)
			if !exists {
				fmt.Println(styles.Note(i18n.T("settings.not_created")))
			}
			return nil
		},
	}
}

func newSettingsResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "reset",
		Short:   i18n.T("cmd.settings.reset.short"),
		Long:    i18n.T("cmd.settings.reset.long"),
		Example: `gofi settings reset`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fresh := settings.Default()
			if err := settings.Save(fresh); err != nil {
				return err
			}
			settings.Use(fresh)
			fmt.Println(styles.Success(i18n.T("settings.reset_done")))
			return runSettingsShow()
		},
	}
}

// parseSettingArgs accepts both `set key value` and `set key=value`.
func parseSettingArgs(args []string) (key, value string, err error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	k, v, ok := strings.Cut(args[0], "=")
	if !ok {
		return "", "", fmt.Errorf("expected <key> <value> or <key>=<value>, got %q", args[0])
	}
	return k, v, nil
}

func runSettingsShow() error {
	s := settings.Active()
	path, exists, err := settings.Locate()
	if err != nil {
		return err
	}
	fileValue := path
	if !exists {
		fileValue = path + "  " + i18n.T("settings.not_created")
	}

	rows := []struct{ label, value string }{
		{i18n.T("settings.label.lang"), languageLabel(s.Language)},
		{i18n.T("settings.label.color"), s.Color},
		{i18n.T("settings.label.output"), s.Output},
		{i18n.T("settings.label.checkin"), boolLabel(s.Checkin)},
		{i18n.T("settings.label.file"), fileValue},
	}
	width := 0
	for _, r := range rows {
		if n := len([]rune(r.label)); n > width {
			width = n
		}
	}

	var b strings.Builder
	b.WriteString(styles.Header(i18n.T("settings.title")) + "\n\n")
	for _, r := range rows {
		pad := strings.Repeat(" ", width-len([]rune(r.label))+2)
		b.WriteString("  " + styles.Label(r.label) + pad + styles.Value(r.value) + "\n")
	}
	fmt.Println()
	fmt.Print(b.String())
	fmt.Println()
	return nil
}

func runSettingsSet(key, value string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	updated := settings.Active().Clone()

	before, err := updated.Get(key)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("settings.unknown_key", key, settingsKeyList()))
	}
	if err := updated.Set(key, value); err != nil {
		if errors.Is(err, settings.ErrInvalidValue) {
			return fmt.Errorf("%s", i18n.T("settings.bad_value", value, key, settingsValueList(key)))
		}
		return err
	}
	after, _ := updated.Get(key)

	// apply before printing, so the confirmation is already in the new language
	settings.Use(updated)
	if before == after {
		fmt.Println(styles.Note(i18n.T("settings.unchanged", key, displayValue(key, after))))
		return nil
	}
	if err := settings.Save(updated); err != nil {
		return err
	}
	fmt.Println(styles.Success(i18n.T("settings.updated", key, displayValue(key, after))))
	fmt.Println(styles.Note(i18n.T("settings.saved", updated.Path())))
	return nil
}

func runSettingsWizard() error {
	if !interactiveTerminal() {
		return errors.New(i18n.T("settings.needs_tty"))
	}
	out, err := wizard.RunSetup(settings.Active())
	if err != nil {
		if errors.Is(err, wizard.ErrSetupCancelled) {
			fmt.Println(styles.Note(i18n.T("setup.cancelled")))
			return nil
		}
		return err
	}
	settings.Use(out)
	if err := settings.Save(out); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println(styles.Success(i18n.T("setup.saved", out.Path())))
	return runSettingsShow()
}

// displayValue renders a stored value for humans: endonym for languages,
// yes/no for booleans.
func displayValue(key, value string) string {
	switch key {
	case settings.KeyLanguage:
		return languageLabel(value)
	case settings.KeyCheckin:
		return boolLabel(value == "true")
	}
	return value
}

func boolLabel(v bool) string {
	if v {
		return i18n.T("setup.checkin.yes")
	}
	return i18n.T("setup.checkin.no")
}
