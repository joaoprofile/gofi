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

	// graph command
	"cmd.graph.short":            "Constrói e consulta o grafo de código deste projeto",
	"cmd.graph.build.short":      "Varre o código e (re)constrói o grafo",
	"cmd.graph.explain.short":    "Descreve um símbolo pelo grafo, sem abrir arquivos",
	"cmd.graph.open.short":       "Abre a visualização do grafo no navegador",
	"cmd.graph.flag.deep":        "resolve as chamadas pelo type-checker: exato, mas o projeto precisa compilar",
	"cmd.graph.flag.fast":        "força o modo sintático mesmo com `graph: deep: true` — é o que os hooks de git usam",
	"cmd.graph.flag.update":      "só reconstrói se algum arquivo .go mudou (barato o bastante para um hook)",
	"cmd.graph.flag.open":        "abre a visualização assim que a construção terminar",
	"cmd.graph.flag.tests":       "inclui os arquivos _test.go no grafo",
	"cmd.graph.flag.no_html":     "não gera a visualização HTML",
	"cmd.graph.flag.exclude":     "padrões de pasta a ignorar (pode repetir)",
	"cmd.graph.flag.max_file_kb": "ignora arquivos maiores que isso, em KB (0 = sem limite)",
	"cmd.graph.flag.verbose":     "mostra os avisos da varredura",
	"cmd.graph.flag.to":          "mostra o caminho até este símbolo em vez de descrever o primeiro",
	"cmd.graph.flag.limit":       "máximo de arestas exibidas por relação",
	"cmd.graph.flag.lang":        "linguagem do grafo (go é nativo; as demais usam um extractor instalado)",
	"cmd.graph.flag.timeout":     "tempo máximo de execução de um extractor externo",
	"cmd.graph.install.short":    "Instala o extractor de uma linguagem não-nativa",
	"cmd.graph.flag.from":        "origem do extractor: um caminho local ou uma URL https",
	"cmd.graph.flag.sha256":      "sha256 esperado do extractor, em hexadecimal",
	"cmd.graph.flag.remove":      "remove o extractor instalado neste projeto",
	"graph.build.unchanged":      "grafo: nenhum arquivo .go mudou, mantido como está (%d arquivos)",
	"graph.build.done":           "grafo: %d pacotes, %d símbolos, %d arestas em %d ms (modo %s)",
	"graph.build.ambiguous":      "grafo: %d chamadas ambíguas no modo rápido — use --deep para resolvê-las",
	"graph.build.more_diags":     "  ... e mais %d avisos do extractor",
	"graph.build.written":        "grafo: escrito em %s/ (%s)",
	"graph.install.none":         "nenhum extractor instalado — use `gofi graph install <linguagem> --from <caminho|url>`",
	"graph.install.list":         "extractors disponíveis para este projeto:",
	"graph.install.done":         "extractor de %s instalado em %s",
	"graph.install.sha256":       "  sha256: %s",
	"graph.install.next":         "  construa o grafo com `gofi graph build --lang %s`",
	"graph.install.removed":      "extractor de %s removido deste projeto",

	"graph.build.scope_done":       "%s: %d pacotes, %d símbolos, %d arestas em %d ms (modo %s)",
	"graph.build.scope_unchanged":  "%s: sem alterações (%d arquivos)",
	"cmd.graph.hooks.short":        "Instala os hooks git que mantêm o grafo em dia",
	"cmd.graph.flag.hooks_install": "escreve o bloco gofi nos hooks gerenciados",
	"graph.hooks.none":             "nenhum hook gofi instalado — use `gofi graph hooks --install`",
	"graph.hooks.list":             "hooks que carregam o bloco gofi:",
	"graph.hooks.done":             "hooks do grafo: %s",
	"graph.hooks.failed":           "hooks do grafo — %v; rode `gofi graph hooks --install`",
	"graph.setup.done":             "grafo: %d nós, %d arestas em %s, modo %s, em %s/",
	"graph.setup.empty":            "grafo — nada pôde ser varrido; rode `gofi graph build` para ver por quê",
	"graph.setup.failed":           "grafo — %v; rode `gofi graph build`",

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
