package i18n

// catalogPT is the Portuguese catalog. Keys missing here fall back to English.
var catalogPT = map[string]string{
	// root
	"root.short": "CLI para o ciclo de vida de projetos gofi",
	"root.long": "gofi é a CLI que cria e gerencia o ciclo de vida dos projetos gofi:\n" +
		"scaffolding do projeto, instalação e treinamento dos agentes, execução dos testes.",
	"root.tip_init":      "Dica: rode `gofi init` para criar um projeto na pasta atual.",
	"root.checkin":       "fonte dos agentes acessível (%s)",
	"root.warning":       "aviso: %v",
	"root.flag.no_color": "desliga as cores mesmo em terminal interativo",
	"root.flag.plain":    "saída em texto puro (sem cores, sem caixas) — útil em CI/pipes",

	// help chrome
	"help.usage":           "USO",
	"help.commands":        "COMANDOS",
	"help.examples":        "EXEMPLOS",
	"help.flags":           "FLAGS",
	"help.related":         "RELACIONADOS",
	"help.hint":            "Rode '%s h <comando>' para a ajuda de um comando específico.",
	"help.row":             "Mostra a ajuda (gofi h <comando> para detalhes)",
	"help.unknown_command": "comando %q desconhecido para o gofi",
	"help.did_you_mean":    "Você quis dizer:",
	"cmd.help.short":       "Mostra a ajuda de qualquer comando",

	// command summaries
	"cmd.version.short":          "Mostra versão, commit e dados do build",
	"cmd.init.short":             "Cria um novo projeto gofi (assistente interativo)",
	"cmd.commit.short":           "Adiciona todas as mudanças e cria um commit",
	"cmd.agent.short":            "Gerencia os agentes (add | remove | list)",
	"cmd.agent.add.short":        "Instala um agente neste projeto",
	"cmd.agent.remove.short":     "Remove um agente deste projeto",
	"cmd.agent.list.short":       "Lista os agentes ativos e disponíveis",
	"cmd.remote.short":           "Gerencia o remote git deste projeto",
	"cmd.remote.add.short":       "Define o remote origin com a URL informada",
	"cmd.remote.show.short":      "Mostra o remote origin configurado",
	"cmd.remote.remove.short":    "Remove o remote origin configurado",
	"cmd.train.short":            "Injeta conhecimento de domínio em um agente",
	"cmd.train.list.short":       "Lista os tópicos de treinamento instalados",
	"cmd.train.show.short":       "Mostra o conteúdo de um tópico instalado",
	"cmd.train.edit.short":       "Abre um tópico instalado no $EDITOR",
	"cmd.train.remove.short":     "Apaga um tópico instalado",
	"cmd.test.short":             "Executa as tarefas de teste da linguagem",
	"cmd.test.list.short":        "Lista as tarefas de teste disponíveis",
	"cmd.update.short":           "Atualiza agentes e SDK para a última versão publicada",
	"cmd.institutional.short":    "Gerencia a base de conhecimento institucional (negócio)",
	"cmd.institutional.up.short": "Substitui .claude/institutional/<projeto>/ a partir do repo da organização",
	"cmd.doctor.short":           "Valida o ambiente local para o gofi",
	"cmd.config.short":           "Edita o .gofi.yaml do projeto",
	"cmd.hsec.short":             "Roda o Horusec (SAST) neste projeto",
	"cmd.hsec.start.short":       "Executa a varredura de segurança",
	"cmd.hsec.install.short":     "Instala a CLI do horusec pelo script oficial",
	"cmd.hsec.list.short":        "Lista os achados da última varredura",
	"cmd.sonar.short":            "Roda a análise SonarQube/SonarCloud neste projeto",
	"cmd.sonar.start.short":      "Executa a análise",
	"cmd.sonar.config.short":     "Gera e imprime o sonar-project.properties sem analisar",
	"cmd.sonar.install.short":    "Mostra como instalar o sonar-scanner",

	// settings command
	"cmd.settings.short":   "Configura a própria CLI (idioma, saída)",
	"cmd.settings.related": "gofi config — edita o .gofi.yaml do projeto atual",
	"cmd.settings.long": "Lê e altera as configurações da própria CLI — as que valem para o gofi\n" +
		"em qualquer lugar, e não para um projeto específico.\n\n" +
		"As configurações ficam em um gofi.json guardado junto do executável do gofi,\n" +
		"de modo que uma instalação portátil leva a configuração consigo. Quando essa\n" +
		"pasta é somente leitura (instalação de sistema), o arquivo cai para\n" +
		"~/.gofi/gofi.json. Use GOFI_SETTINGS para fixar um caminho explícito.\n\n" +
		"O arquivo é criado na primeira execução interativa, por um assistente curto.\n" +
		"Rode `gofi settings wizard` para passar por ele de novo.",
	"cmd.settings.show.short": "Mostra as configurações ativas da CLI",
	"cmd.settings.show.long": "Mostra cada configuração da CLI com o valor atual, além do caminho do\n" +
		"gofi.json de onde esses valores vieram.",
	"cmd.settings.set.short": "Altera uma configuração",
	"cmd.settings.set.long": "Altera uma única configuração da CLI e grava no gofi.json.\n\n" +
		"Chaves:\n" +
		"  language  en | pt | fr\n" +
		"  color     auto | always | never\n" +
		"  output    rich | plain\n" +
		"  checkin   true | false",
	"cmd.settings.wizard.short": "Roda de novo o assistente de configuração da CLI",
	"cmd.settings.wizard.long": "Reabre o assistente da primeira execução, já preenchido com os valores\n" +
		"atuais, e grava o resultado no gofi.json.",
	"cmd.settings.path.short": "Mostra o caminho do arquivo de configuração",
	"cmd.settings.path.long": "Mostra o caminho do gofi.json que a CLI lê e se ele já existe.\n" +
		"Útil em scripts ou quando há vários binários do gofi instalados.",
	"cmd.settings.reset.short": "Restaura as configurações padrão da CLI",
	"cmd.settings.reset.long": "Sobrescreve o gofi.json com os padrões (idioma detectado pelo ambiente).\n" +
		"Os arquivos .gofi.yaml dos projetos não são tocados.",

	"settings.title":         "Configurações da CLI",
	"settings.label.lang":    "Idioma",
	"settings.label.color":   "Cores",
	"settings.label.output":  "Saída",
	"settings.label.checkin": "Verificação na inicialização",
	"settings.label.file":    "Arquivo",
	"settings.not_created":   "ainda não criado — rode `gofi settings wizard`",
	"settings.saved":         "Gravado em %s",
	"settings.updated":       "%s agora é %s.",
	"settings.unchanged":     "%s já é %s; nada a fazer.",
	"settings.unknown_key":   "configuração %q desconhecida (chaves válidas: %s)",
	"settings.bad_value":     "valor %q inválido para %s (válidos: %s)",
	"settings.reset_done":    "Configurações restauradas para os padrões.",
	"settings.needs_tty":     "este comando precisa de um terminal interativo",

	// first-run setup wizard
	"setup.welcome.title": "Bem-vindo ao gofi",
	"setup.welcome.desc": "Esta é a primeira execução nesta máquina. Vamos configurar a CLI — leva\n" +
		"poucos segundos e fica gravado no gofi.json, junto do executável.",
	"setup.language.title": "Idioma",
	"setup.language.desc":  "Idioma usado nas mensagens e na ajuda da CLI.",
	"setup.prefs.title":    "Preferências",
	"setup.prefs.desc":     "Como o gofi desenha a saída neste terminal.",
	"setup.color.title":    "Cores",
	"setup.color.desc":     "Automático segue o terminal (desliga em pipe ou com NO_COLOR definido).",
	"setup.color.auto":     "Automático — segue o terminal",
	"setup.color.always":   "Sempre — mantém as cores mesmo em pipe",
	"setup.color.never":    "Nunca — monocromático",
	"setup.output.title":   "Estilo da saída",
	"setup.output.desc":    "Completo desenha logo, caixas e spinners. Simples imprime só texto.",
	"setup.output.rich":    "Completo — logo, painéis e spinners",
	"setup.output.plain":   "Simples — só texto, ideal para CI e pipes",
	"setup.checkin.title":  "Verificar a fonte das skills ao iniciar?",
	"setup.checkin.desc":   "Rodar `gofi` sem argumentos dentro de um projeto consulta o repositório configurado para confirmar que ele está acessível.",
	"setup.checkin.yes":    "Sim",
	"setup.checkin.no":     "Não",
	"setup.apply.title":    "Salvar esta configuração?",
	"setup.apply.desc":     "Gravada no gofi.json. Você muda quando quiser com `gofi settings`.",
	"setup.apply.affirm":   "Salvar",
	"setup.apply.negative": "Cancelar",
	"setup.cancelled":      "Configuração cancelada — usando os padrões nesta execução.",
	"setup.saved":          "Configuração gravada em %s",
	"setup.save_failed":    "aviso: não foi possível gravar a configuração (%v) — ela vale só nesta execução.",
	"setup.next":           "Rode `gofi settings` para revisar, ou `gofi init` para criar um projeto.",
}
