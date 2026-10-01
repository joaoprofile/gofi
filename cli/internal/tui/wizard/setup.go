package wizard

import (
	"errors"
	"fmt"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/settings"
	"github.com/joaoprofile/gofi/cli/internal/tui/flow"
)

// ErrSetupCancelled is returned when the user quits or declines the final
// confirm.
var ErrSetupCancelled = errors.New("setup cancelled")

// RunSetup asks the CLI preferences: on the first run of gofi, and again on
// `gofi settings wizard`.
//
// It runs in two dialogues on purpose. The language is picked first, then the
// remaining questions render in that language, so the choice is visible right
// away. initial pre-populates the answers.
func RunSetup(initial *settings.Settings) (*settings.Settings, error) {
	if initial == nil {
		initial = settings.Default()
	}
	r := initial.Clone()
	previous := i18n.Current()
	fail := func(err error) (*settings.Settings, error) {
		i18n.SetLanguage(previous)
		if errors.Is(err, flow.ErrCancelled) {
			return nil, ErrSetupCancelled
		}
		return nil, err
	}

	// step 1: language, asked in the language we currently believe in
	i18n.SetLanguage(r.Language)
	header := flow.Header{
		Title: i18n.T("setup.welcome.title"),
		Extra: strings.Split(i18n.T("setup.welcome.desc"), "\n"),
	}
	if err := flow.Run(header, []flow.Step{{
		Kind:    flow.Select,
		Title:   i18n.T("setup.language.title"),
		Help:    i18n.T("setup.language.desc"),
		Options: languageOptions(),
		Choice:  &r.Language,
	}}); err != nil {
		return fail(err)
	}

	// step 2: the rest, now speaking the chosen language
	i18n.SetLanguage(r.Language)
	proceed := true
	steps := []flow.Step{
		{
			Kind:  flow.Select,
			Title: i18n.T("setup.color.title"),
			Help:  i18n.T("setup.color.desc"),
			Options: []flow.Option{
				{Label: i18n.T("setup.color.auto"), Value: settings.ColorAuto},
				{Label: i18n.T("setup.color.always"), Value: settings.ColorAlways},
				{Label: i18n.T("setup.color.never"), Value: settings.ColorNever},
			},
			Choice: &r.Color,
		},
		{
			Kind:  flow.Select,
			Title: i18n.T("setup.output.title"),
			Help:  i18n.T("setup.output.desc"),
			Options: []flow.Option{
				{Label: i18n.T("setup.output.rich"), Value: settings.OutputRich},
				{Label: i18n.T("setup.output.plain"), Value: settings.OutputPlain},
			},
			Choice: &r.Output,
		},
		{
			Kind:        flow.Confirm,
			Title:       i18n.T("setup.checkin.title"),
			Help:        i18n.T("setup.checkin.desc"),
			Affirmative: i18n.T("setup.checkin.yes"),
			Negative:    i18n.T("setup.checkin.no"),
			Bool:        &r.Checkin,
		},
		{
			Kind:        flow.Confirm,
			Title:       i18n.T("setup.apply.title"),
			Help:        i18n.T("setup.apply.desc"),
			Affirmative: i18n.T("setup.apply.affirm"),
			Negative:    i18n.T("setup.apply.negative"),
			Bool:        &proceed,
		},
	}
	if err := flow.Run(flow.Header{}, steps); err != nil {
		return fail(err)
	}
	if !proceed {
		i18n.SetLanguage(previous)
		return nil, ErrSetupCancelled
	}
	return r, nil
}

// languageOptions lists the languages by endonym, with the English name
// alongside so a user stuck on an unfamiliar language still finds their own.
func languageOptions() []flow.Option {
	langs := i18n.Supported()
	out := make([]flow.Option, 0, len(langs))
	for _, l := range langs {
		label := l.Native
		if l.Native != l.Name {
			label = fmt.Sprintf("%s (%s)", l.Native, l.Name)
		}
		out = append(out, flow.Option{Label: label, Value: l.Code})
	}
	return out
}
