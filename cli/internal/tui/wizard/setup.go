package wizard

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/settings"
	"github.com/joaoprofile/gofi-cli/internal/tui/styles"
)

// ErrSetupCancelled is returned when the user declines the final confirm.
var ErrSetupCancelled = errors.New("setup cancelled")

// RunSetup shows the CLI setup form: on the first run of gofi, and again on
// `gofi settings wizard`.
//
// It runs in two steps on purpose. The language is picked first, then the
// remaining questions render in that language, so the choice is visible right
// away. initial pre-populates the form.
func RunSetup(initial *settings.Settings) (*settings.Settings, error) {
	if initial == nil {
		initial = settings.Default()
	}
	r := initial.Clone()
	previous := i18n.Current()

	// step 1: language, rendered in the language we currently believe in
	i18n.SetLanguage(r.Language)
	langForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(i18n.T("setup.welcome.title")).
				Description(i18n.T("setup.welcome.desc")),
			huh.NewSelect[string]().
				Title(i18n.T("setup.language.title")).
				Description(i18n.T("setup.language.desc")).
				Options(languageOptions()...).
				Value(&r.Language),
		),
	).WithTheme(styles.FormTheme()).WithAccessible(!styles.Enabled())

	if err := langForm.Run(); err != nil {
		i18n.SetLanguage(previous)
		return nil, err
	}

	// step 2: the rest, now speaking the chosen language
	i18n.SetLanguage(r.Language)
	proceed := true
	prefsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(i18n.T("setup.prefs.title")).
				Description(i18n.T("setup.prefs.desc")),
			huh.NewSelect[string]().
				Title(i18n.T("setup.color.title")).
				Description(i18n.T("setup.color.desc")).
				Options(
					huh.NewOption(i18n.T("setup.color.auto"), settings.ColorAuto),
					huh.NewOption(i18n.T("setup.color.always"), settings.ColorAlways),
					huh.NewOption(i18n.T("setup.color.never"), settings.ColorNever),
				).
				Value(&r.Color),
			huh.NewSelect[string]().
				Title(i18n.T("setup.output.title")).
				Description(i18n.T("setup.output.desc")).
				Options(
					huh.NewOption(i18n.T("setup.output.rich"), settings.OutputRich),
					huh.NewOption(i18n.T("setup.output.plain"), settings.OutputPlain),
				).
				Value(&r.Output),
			huh.NewConfirm().
				Title(i18n.T("setup.checkin.title")).
				Description(i18n.T("setup.checkin.desc")).
				Affirmative(i18n.T("setup.checkin.yes")).
				Negative(i18n.T("setup.checkin.no")).
				Value(&r.Checkin),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title(i18n.T("setup.apply.title")).
				Description(i18n.T("setup.apply.desc")).
				Affirmative(i18n.T("setup.apply.affirm")).
				Negative(i18n.T("setup.apply.negative")).
				Value(&proceed),
		),
	).WithTheme(styles.FormTheme()).WithAccessible(!styles.Enabled())

	if err := prefsForm.Run(); err != nil {
		i18n.SetLanguage(previous)
		return nil, err
	}
	if !proceed {
		i18n.SetLanguage(previous)
		return nil, ErrSetupCancelled
	}
	return r, nil
}

// languageOptions lists the languages by endonym, with the English name
// alongside so a user stuck on an unfamiliar language still finds their own.
func languageOptions() []huh.Option[string] {
	langs := i18n.Supported()
	out := make([]huh.Option[string], 0, len(langs))
	for _, l := range langs {
		label := l.Native
		if l.Native != l.Name {
			label = fmt.Sprintf("%s (%s)", l.Native, l.Name)
		}
		out = append(out, huh.NewOption(label, l.Code))
	}
	return out
}
