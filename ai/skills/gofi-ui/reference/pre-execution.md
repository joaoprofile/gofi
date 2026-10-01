# Pré-execução obrigatória (detalhe)

## Pré-execução obrigatória

Antes de qualquer linha de código:

1. Ler `.gofi.yaml` (raiz) — extrair `project.name` e o **bloco de UI** (`frontend:`
   ou `ui:`) conforme o **§Bloco de UI** (`reference/ds-resolution.md`): forma única
   (`framework`/`path`/`ds`/`styling`/`state`/`testing`/`brand`) ou multi-superfície
   (`ui.web`/`ui.mobile`). Derivar a(s) **superfície(s)-alvo** (web e/ou mobile pelo
   `framework`) e, **para cada uma, o `<ds>`** = valor da chave `ds` — o **nome da
   pasta do DS não sai da skill, sai daqui**. O bloco de UI **coexiste** com o backend
   (`project.language`) num full-stack. Se o bloco de UI **ou a chave `ds`** não
   existir, **pare e peça ao usuário** para configurá-los (a skill não adivinha o DS).
   Também leia `styling`/`state`/`testing` — a forma da implementação vem daí + dos
   docs do DS, **não** de presunção. Se `brand` não existir, execute o **Bootstrap de
   marca** (`reference/brand-bootstrap.md`) **antes de qualquer código**.
2. O `AGENTS.md` da raiz (já carregado) — mapa de paths físicos do projeto
3. Ler `.claude/memory/project.md` — visão global, serviços e convenções (índice de contextos: `/gofi-status`)
4. Ler `.claude/memory/contexts/{contexto}.md` se existir — handoff do
   `gofi-spec` e do `gofi-eng` (contratos de API, rotas, DTOs)
4b. **O front também tem grafo — consulte-o antes de varrer a árvore.** Cada
   superfície declarada no `.gofi.yaml` é um **escopo** com grafo próprio, lido
   pelo extractor TS/JS: `.gofi/index/code/{nome-do-bloco}/` (`frontend`, `mobile`,
   ou a chave em `surfaces:` — é o nome do bloco de config, não o `<surface>`
   dos paths do DS). Escada: `.gofi/index/code/gofi_graph_index.json` (quais escopos,
   em que pasta, que framework, que modo) → o `gofi_graph_report.md` daquele
   escopo (componentes centrais, comunidades, conexões inesperadas) →
   `gofi show <Componente>` para saber **quem usa** um componente antes
   de alterá-lo, e `gofi find --in code "<termo> <termo>"` para achá-lo quando você
   só tem a descrição. **Sem `--lang`** — a superfície já é escopo do índice
   principal, e `--lang typescript` aponta para uma pasta que não existe.
   **Nunca** abra `gofi_graph.json`; `gofi find --text`/`--regex` só para o que não é símbolo
   (classe de CSS, chave de i18n, texto). Limites a declarar: o extractor TS/JS
   **não** lê `//gofi:context` (o campo vem vazio — ali a ponte para a spec ainda
   é o nome da pasta) e o escopo só existe se o `path` da superfície existir no
   disco. Protocolo: `.claude/expertise/harness-protocols/graph-retrieval.md`
5. Ler a spec — **fonte da verdade**. **Procurar documento é `gofi find`:** `gofi find "<o que você precisa saber>"` — devolve o documento, a §seção e a **faixa de linhas**; leia com `Read(offset, limit)`, nunca o arquivo inteiro. Vazio quase sempre é vocabulário, não documento faltando: tente o termo técnico e registre o par que faltou em `.claude/lexicon/sinonimos.md`. Os `INDEX.md` servem para **navegar** um contexto, não para **achar** um assunto. Protocolo: `.claude/expertise/harness-protocols/rag-retrieval.md`
6. Ler **knowledge cross-agent**: `.claude/knowledge/INDEX.md` (núcleo ⬤ + só os módulos que a tarefa pede) (inclui `expertise/diagramming/conventions.md` — jornada do usuário e fluxos de UX devem ser PlantUML)
7. Ler **UX e design**: `.claude/expertise/ui-design/` (princípios e regras de UX, theming) +
   `.claude/knowledge/ui/*.md` — aprendizado do time, que vence o pack quando diverge
8. **Tokens (sempre):** ler `.claude/expertise/ui-design/design-tokens.md` — estrutura de
   tokens/escalas e como as **cores do projeto** preenchem os papéis (sem paleta fixa).
9. Para **cada superfície-alvo** (`<surface>` = `web` e/ou `mobile`, derivada do
   `framework`; **`<ds>` = valor da chave `ds` lida no passo 1**, nunca um nome fixo) —
   repita a leitura abaixo. Comece pelo índice de retrieval quando existir:
   - Ler o **índice RAG** da superfície se houver: `.claude/sdk/<surface>/INDEX.md`
     (descobre docs por `keywords` → leia só o frontmatter do alvo → só a §relevante).
   - Ler o **manifesto do DS** — o `.md` na **raiz** de `.claude/sdk/<surface>/<ds>/`
     (é o único doc no topo da pasta; foundations/components/patterns são subpastas).
     **Não** assuma um nome de arquivo específico; descubra pelo INDEX ou listando a raiz.
   - Ler **foundations** pertinentes:
     `.claude/sdk/<surface>/<ds>/foundations/{tokens-*,color,typography,
     spacing-layout,radius-elevation,motion,accessibility,...}.md`
   - Ler o **catálogo** e usar componentes existentes antes de criar:
     `.claude/sdk/<surface>/<ds>/components/{_index,...}.md`
   - Ler os **patterns** da tela alvo:
     `.claude/sdk/<surface>/<ds>/patterns/{states,app-shell|navigation,
     page-templates,forms,feedback,...}.md`
   - Ler as **regras de código + estrutura** da superfície:
     `.claude/sdk/<surface>/knowledge/absolute-rules.md` e `structure.md`
     (framework-specific), e usar os esqueletos em
     `.claude/sdk/<surface>/boilerplates/*.md` **antes** de implementar. **Importe do
     que o DS expõe (lib npm OU componentes do repo, conforme o manifesto) — não o recrie.**
   - **Se o contexto pertence a uma área/app com DS próprio**
     (app-specific — ex.: um back-office/admin distinto do front principal):
     ler o DS app-specific em `.claude/sdk/<surface>/<app>.md` — é o
     template a seguir ali. **Não confundir** com o DS principal — cada app tem o seu.
10. Verificar se já existem arquivos no path da feature/page —
   **nunca sobrescrever sem confirmar**

> Se a spec for ambígua, contradizer um padrão das `absolute-rules` ou não
> mencionar estados de UI (loading/empty/error/success), pare e pergunte.
> Nunca infira UX.
