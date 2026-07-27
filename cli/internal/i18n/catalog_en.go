package i18n

// catalogEN is the reference catalog. Every key the CLI uses must exist here;
// the other catalogs are checked against it by TestCatalogsAreComplete.
var catalogEN = map[string]string{
	// root
	"root.short": "CLI for gofi project lifecycle",
	"root.long": "gofi is the CLI tool that bootstraps and manages the lifecycle of gofi projects:\n" +
		"project scaffolding, agent installation and training, test execution.",
	"root.tip_init":      "Tip: run `gofi init` to bootstrap a project in the current directory.",
	"root.checkin":       "agents source reachable (%s)",
	"root.warning":       "warning: %v",
	"root.flag.no_color": "disable color output even on TTY",
	"root.flag.plain":    "plain text output (no colors, no boxes) — useful for CI/pipes",

	// help chrome
	"help.usage":           "USAGE",
	"help.commands":        "COMMANDS",
	"help.examples":        "EXAMPLES",
	"help.flags":           "FLAGS",
	"help.related":         "RELATED",
	"help.hint":            "Run '%s h <command>' for command-specific help.",
	"help.row":             "Show help (gofi h <command> for details)",
	"help.unknown_command": "unknown command %q for gofi",
	"help.did_you_mean":    "Did you mean:",
	"cmd.help.short":       "Show help for any command",

	// command summaries
	"cmd.version.short":          "Print version, commit and build info",
	"cmd.init.short":             "Bootstrap a new gofi project (interactive wizard)",
	"cmd.commit.short":           "Stage all changes and create a commit",
	"cmd.agent.short":            "Manage agents (add | remove | list)",
	"cmd.agent.add.short":        "Install an agent into this project",
	"cmd.agent.remove.short":     "Uninstall an agent from this project",
	"cmd.agent.list.short":       "List active and available agents",
	"cmd.remote.short":           "Manage the git remote for this project",
	"cmd.remote.add.short":       "Set the origin remote to the given URL",
	"cmd.remote.show.short":      "Print the configured origin remote",
	"cmd.remote.remove.short":    "Remove the configured origin remote",
	"cmd.train.short":            "Inject domain knowledge into an agent",
	"cmd.train.list.short":       "List installed training topics",
	"cmd.train.show.short":       "Print the contents of an installed topic",
	"cmd.train.edit.short":       "Open an installed topic in $EDITOR",
	"cmd.train.remove.short":     "Delete an installed topic",
	"cmd.test.short":             "Run language test tasks",
	"cmd.test.list.short":        "List available test tasks",
	"cmd.update.short":           "Update agents and SDK to the latest tagged version",
	"cmd.institutional.short":    "Manage the institutional (business) knowledge base",
	"cmd.institutional.up.short": "Full-replace .claude/institutional/<project>/ from the org repo",
	"cmd.doctor.short":           "Validate the local environment for gofi",
	"cmd.config.short":           "Edit the project's .gofi.yaml",
	"cmd.hsec.short":             "Run Horusec SAST against this project",
	"cmd.hsec.start.short":       "Run the security scan",
	"cmd.hsec.install.short":     "Install the horusec CLI via the official script",
	"cmd.hsec.list.short":        "List findings from the last scan",
	"cmd.sonar.short":            "Run SonarQube/SonarCloud analysis against this project",
	"cmd.sonar.start.short":      "Run the analysis",
	"cmd.sonar.config.short":     "Render and print sonar-project.properties without scanning",
	"cmd.sonar.install.short":    "Print instructions to install sonar-scanner",

	// settings command
	"cmd.settings.short":   "Configure the CLI itself (language, output)",
	"cmd.settings.related": "gofi config — edit the current project's .gofi.yaml",
	"cmd.settings.long": "Read and change the CLI's own settings — the ones that apply to gofi\n" +
		"everywhere, not to a single project.\n\n" +
		"Settings live in a gofi.json stored next to the gofi executable, so a\n" +
		"portable install carries its configuration with it. When that folder is\n" +
		"read-only (a system-wide install), the file falls back to ~/.gofi/gofi.json.\n" +
		"Set GOFI_SETTINGS to pin an explicit path.\n\n" +
		"The file is created on the first interactive run, through a short setup\n" +
		"wizard. Run `gofi settings wizard` to go through it again.",
	"cmd.settings.show.short": "Print the active CLI settings",
	"cmd.settings.show.long": "Print every CLI setting with its current value, plus the path of the\n" +
		"gofi.json those values came from.",
	"cmd.settings.set.short": "Change one setting",
	"cmd.settings.set.long": "Change a single CLI setting and persist it to gofi.json.\n\n" +
		"Keys:\n" +
		"  language  en | pt | fr\n" +
		"  color     auto | always | never\n" +
		"  output    rich | plain\n" +
		"  checkin   true | false",
	"cmd.settings.wizard.short": "Re-run the interactive CLI setup",
	"cmd.settings.wizard.long": "Reopen the first-run setup wizard, pre-populated with the current values,\n" +
		"and write the result back to gofi.json.",
	"cmd.settings.path.short": "Print the settings file path",
	"cmd.settings.path.long": "Print the path of the gofi.json the CLI reads, and whether it exists yet.\n" +
		"Useful when scripting or when several gofi binaries are installed.",
	"cmd.settings.reset.short": "Restore the default CLI settings",
	"cmd.settings.reset.long": "Overwrite gofi.json with the defaults (language detected from the\n" +
		"environment). The project's .gofi.yaml files are not touched.",

	"settings.title":         "CLI settings",
	"settings.label.lang":    "Language",
	"settings.label.color":   "Color",
	"settings.label.output":  "Output",
	"settings.label.checkin": "Startup check",
	"settings.label.file":    "File",
	"settings.not_created":   "not created yet — run `gofi settings wizard`",
	"settings.saved":         "Saved %s",
	"settings.updated":       "%s is now %s.",
	"settings.unchanged":     "%s is already %s; nothing to do.",
	"settings.unknown_key":   "unknown setting %q (valid keys: %s)",
	"settings.bad_value":     "invalid value %q for %s (valid: %s)",
	"settings.reset_done":    "Settings restored to defaults.",
	"settings.needs_tty":     "this command needs an interactive terminal",

	// first-run setup wizard
	"setup.welcome.title": "Welcome to gofi",
	"setup.welcome.desc": "This is the first run on this machine. Let's configure the CLI — it takes\n" +
		"a few seconds and is stored in gofi.json next to the executable.",
	"setup.language.title": "Language",
	"setup.language.desc":  "Language used by the CLI's messages and help.",
	"setup.prefs.title":    "Preferences",
	"setup.prefs.desc":     "How gofi renders its output in this terminal.",
	"setup.color.title":    "Colors",
	"setup.color.desc":     "Auto follows the terminal (disabled when piped or when NO_COLOR is set).",
	"setup.color.auto":     "Auto — follow the terminal",
	"setup.color.always":   "Always — keep colors even when piped",
	"setup.color.never":    "Never — plain monochrome",
	"setup.output.title":   "Output style",
	"setup.output.desc":    "Rich draws the logo, boxes and spinners. Plain prints text only.",
	"setup.output.rich":    "Rich — logo, panels and spinners",
	"setup.output.plain":   "Plain — text only, friendly to CI and pipes",
	"setup.checkin.title":  "Check the skills source on startup?",
	"setup.checkin.desc":   "Running plain `gofi` inside a project pings the configured repository to confirm it is reachable.",
	"setup.checkin.yes":    "Yes",
	"setup.checkin.no":     "No",
	"setup.apply.title":    "Save this configuration?",
	"setup.apply.desc":     "Written to gofi.json. Change it any time with `gofi settings`.",
	"setup.apply.affirm":   "Save",
	"setup.apply.negative": "Cancel",
	"setup.cancelled":      "Setup cancelled — using the defaults for this run.",
	"setup.saved":          "Settings saved to %s",
	"setup.save_failed":    "warning: could not save settings (%v) — using them for this run only.",
	"setup.next":           "Run `gofi settings` to review, or `gofi init` to bootstrap a project.",
}
