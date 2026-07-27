package i18n

// catalogFR is the French catalog. Keys missing here fall back to English.
var catalogFR = map[string]string{
	// root
	"root.short": "CLI pour le cycle de vie des projets gofi",
	"root.long": "gofi est la CLI qui initialise et gère le cycle de vie des projets gofi :\n" +
		"génération du projet, installation et entraînement des agents, exécution des tests.",
	"root.tip_init":      "Astuce : lancez `gofi init` pour initialiser un projet dans le dossier courant.",
	"root.checkin":       "source des agents joignable (%s)",
	"root.warning":       "avertissement : %v",
	"root.flag.no_color": "désactive les couleurs même dans un terminal",
	"root.flag.plain":    "sortie en texte brut (sans couleurs ni cadres) — utile en CI/pipes",

	// help chrome
	"help.usage":           "UTILISATION",
	"help.commands":        "COMMANDES",
	"help.examples":        "EXEMPLES",
	"help.flags":           "OPTIONS",
	"help.related":         "VOIR AUSSI",
	"help.hint":            "Lancez '%s h <commande>' pour l'aide d'une commande précise.",
	"help.row":             "Affiche l'aide (gofi h <commande> pour le détail)",
	"help.unknown_command": "commande %q inconnue pour gofi",
	"help.did_you_mean":    "Vouliez-vous dire :",
	"cmd.help.short":       "Affiche l'aide de n'importe quelle commande",

	// command summaries
	"cmd.version.short":          "Affiche la version, le commit et les infos de build",
	"cmd.init.short":             "Initialise un nouveau projet gofi (assistant interactif)",
	"cmd.commit.short":           "Indexe toutes les modifications et crée un commit",
	"cmd.agent.short":            "Gère les agents (add | remove | list)",
	"cmd.agent.add.short":        "Installe un agent dans ce projet",
	"cmd.agent.remove.short":     "Désinstalle un agent de ce projet",
	"cmd.agent.list.short":       "Liste les agents actifs et disponibles",
	"cmd.remote.short":           "Gère le dépôt distant git de ce projet",
	"cmd.remote.add.short":       "Définit le dépôt distant origin à l'URL donnée",
	"cmd.remote.show.short":      "Affiche le dépôt distant origin configuré",
	"cmd.remote.remove.short":    "Supprime le dépôt distant origin configuré",
	"cmd.train.short":            "Injecte de la connaissance métier dans un agent",
	"cmd.train.list.short":       "Liste les sujets d'entraînement installés",
	"cmd.train.show.short":       "Affiche le contenu d'un sujet installé",
	"cmd.train.edit.short":       "Ouvre un sujet installé dans $EDITOR",
	"cmd.train.remove.short":     "Supprime un sujet installé",
	"cmd.test.short":             "Exécute les tâches de test du langage",
	"cmd.test.list.short":        "Liste les tâches de test disponibles",
	"cmd.update.short":           "Met à jour les agents et le SDK vers la dernière version publiée",
	"cmd.institutional.short":    "Gère la base de connaissance institutionnelle (métier)",
	"cmd.institutional.up.short": "Remplace .claude/institutional/<projet>/ depuis le dépôt de l'organisation",
	"cmd.doctor.short":           "Vérifie l'environnement local pour gofi",
	"cmd.config.short":           "Modifie le .gofi.yaml du projet",
	"cmd.hsec.short":             "Lance Horusec (SAST) sur ce projet",
	"cmd.hsec.start.short":       "Lance l'analyse de sécurité",
	"cmd.hsec.install.short":     "Installe la CLI horusec via le script officiel",
	"cmd.hsec.list.short":        "Liste les résultats de la dernière analyse",
	"cmd.sonar.short":            "Lance l'analyse SonarQube/SonarCloud sur ce projet",
	"cmd.sonar.start.short":      "Lance l'analyse",
	"cmd.sonar.config.short":     "Génère et affiche sonar-project.properties sans analyser",
	"cmd.sonar.install.short":    "Affiche comment installer sonar-scanner",

	// settings command
	"cmd.settings.short":   "Configure la CLI elle-même (langue, sortie)",
	"cmd.settings.related": "gofi config — modifie le .gofi.yaml du projet courant",
	"cmd.settings.long": "Lit et modifie les réglages de la CLI elle-même — ceux qui s'appliquent à\n" +
		"gofi partout, et non à un projet donné.\n\n" +
		"Les réglages vivent dans un gofi.json rangé à côté de l'exécutable gofi :\n" +
		"une installation portable emporte ainsi sa configuration. Si ce dossier est\n" +
		"en lecture seule (installation système), le fichier bascule vers\n" +
		"~/.gofi/gofi.json. Utilisez GOFI_SETTINGS pour fixer un chemin explicite.\n\n" +
		"Le fichier est créé au premier lancement interactif, via un court assistant.\n" +
		"Lancez `gofi settings wizard` pour le refaire.",
	"cmd.settings.show.short": "Affiche les réglages actifs de la CLI",
	"cmd.settings.show.long": "Affiche chaque réglage de la CLI avec sa valeur courante, ainsi que le\n" +
		"chemin du gofi.json dont ces valeurs proviennent.",
	"cmd.settings.set.short": "Modifie un réglage",
	"cmd.settings.set.long": "Modifie un seul réglage de la CLI et l'enregistre dans gofi.json.\n\n" +
		"Clés :\n" +
		"  language  en | pt | fr\n" +
		"  color     auto | always | never\n" +
		"  output    rich | plain\n" +
		"  checkin   true | false",
	"cmd.settings.wizard.short": "Relance l'assistant de configuration de la CLI",
	"cmd.settings.wizard.long": "Rouvre l'assistant du premier lancement, prérempli avec les valeurs\n" +
		"courantes, puis écrit le résultat dans gofi.json.",
	"cmd.settings.path.short": "Affiche le chemin du fichier de réglages",
	"cmd.settings.path.long": "Affiche le chemin du gofi.json que la CLI lit, et s'il existe déjà.\n" +
		"Pratique en script ou quand plusieurs binaires gofi sont installés.",
	"cmd.settings.reset.short": "Rétablit les réglages par défaut de la CLI",
	"cmd.settings.reset.long": "Écrase gofi.json avec les valeurs par défaut (langue détectée depuis\n" +
		"l'environnement). Les fichiers .gofi.yaml des projets ne sont pas touchés.",

	"settings.title":         "Réglages de la CLI",
	"settings.label.lang":    "Langue",
	"settings.label.color":   "Couleurs",
	"settings.label.output":  "Sortie",
	"settings.label.checkin": "Vérification au démarrage",
	"settings.label.file":    "Fichier",
	"settings.not_created":   "pas encore créé — lancez `gofi settings wizard`",
	"settings.saved":         "Enregistré dans %s",
	"settings.updated":       "%s vaut désormais %s.",
	"settings.unchanged":     "%s vaut déjà %s ; rien à faire.",
	"settings.unknown_key":   "réglage %q inconnu (clés valides : %s)",
	"settings.bad_value":     "valeur %q invalide pour %s (valides : %s)",
	"settings.reset_done":    "Réglages rétablis par défaut.",
	"settings.needs_tty":     "cette commande requiert un terminal interactif",

	// first-run setup wizard
	"setup.welcome.title": "Bienvenue dans gofi",
	"setup.welcome.desc": "C'est le premier lancement sur cette machine. Configurons la CLI — cela\n" +
		"prend quelques secondes et sera écrit dans gofi.json, à côté de l'exécutable.",
	"setup.language.title": "Langue",
	"setup.language.desc":  "Langue des messages et de l'aide de la CLI.",
	"setup.prefs.title":    "Préférences",
	"setup.prefs.desc":     "La façon dont gofi affiche sa sortie dans ce terminal.",
	"setup.color.title":    "Couleurs",
	"setup.color.desc":     "Auto suit le terminal (désactivé en pipe ou si NO_COLOR est défini).",
	"setup.color.auto":     "Auto — suit le terminal",
	"setup.color.always":   "Toujours — garde les couleurs même en pipe",
	"setup.color.never":    "Jamais — monochrome",
	"setup.output.title":   "Style de sortie",
	"setup.output.desc":    "Riche dessine le logo, les cadres et les spinners. Sobre n'affiche que du texte.",
	"setup.output.rich":    "Riche — logo, cadres et spinners",
	"setup.output.plain":   "Sobre — texte seul, adapté à la CI et aux pipes",
	"setup.checkin.title":  "Vérifier la source des skills au démarrage ?",
	"setup.checkin.desc":   "Lancer `gofi` sans argument dans un projet interroge le dépôt configuré pour confirmer qu'il est joignable.",
	"setup.checkin.yes":    "Oui",
	"setup.checkin.no":     "Non",
	"setup.apply.title":    "Enregistrer cette configuration ?",
	"setup.apply.desc":     "Écrite dans gofi.json. Modifiable à tout moment avec `gofi settings`.",
	"setup.apply.affirm":   "Enregistrer",
	"setup.apply.negative": "Annuler",
	"setup.cancelled":      "Configuration annulée — valeurs par défaut pour ce lancement.",
	"setup.saved":          "Réglages enregistrés dans %s",
	"setup.save_failed":    "avertissement : impossible d'enregistrer les réglages (%v) — ils ne valent que pour ce lancement.",
	"setup.next":           "Lancez `gofi settings` pour vérifier, ou `gofi init` pour initialiser un projet.",
}
