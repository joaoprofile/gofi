# Gofi Ecosystem — Harness

> Repositório-fonte consumido pela CLI [`gofi`](#instalação-da-cli) para gerar e
> manter projetos. Nele vivem os **agents**, **boilerplates**, **SDK docs**,
> **knowledge** e **memória** que formam o *harness* sobre o qual a IA opera.

---

## Instalação da CLI

### Linux / macOS

```sh
curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.ps1 | iex
```

### Versão específica

```sh
# Linux / macOS
GOFI_VERSION=v0.2.0 curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh

# Windows
$env:GOFI_VERSION = "v0.2.0"
iwr -useb https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.ps1 | iex
```

### Diretório de instalação customizado (Linux / macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh -s -- --bin-dir /opt/gofi/bin
```

### Onde o binário é instalado

| OS | Local padrão | Observações |
|----|--------------|-------------|
| Linux / macOS | `/usr/local/bin/gofi` se gravável, senão `$HOME/.local/bin/gofi` | avisa sobre o PATH se `$HOME/.local/bin` não estiver em `$PATH` |
| Windows | `%LOCALAPPDATA%\Programs\gofi\bin\gofi.exe` | adiciona o diretório ao PATH do usuário (sem admin) |

> O instalador **sempre** verifica o SHA-256 contra `checksums.txt` antes de extrair.

Depois de instalar, rode `gofi h` para começar.

---

## DDA = SDD + SDK + Padrões + Boilerplates + Knowledge + Context

**DDA** é a fórmula. Cada parcela resolve uma classe de erro recorrente da IA
quando ela escreve código de produção:

```
DDA  =  SDD                  ← o QUE construir (especificação formal)
      + SDK                  ← o COMO construir (única fonte de verdade)
      + Padrões Arquiteturais ← Clean Arch, Hexagonal, CQRS quando couber
      + SOLID                ← invariantes de design no código gerado
      + DDD                  ← linguagem ubíqua, bounded contexts, agregados
      + Boilerplates         ← a ESTRUTURA por camada (model, repo, service…)
      + Knowledge            ← erros recorrentes virados em regra escrita
      + Context (MCP Light)  ← acesso a DB/API/infra via SDK + docs, sem MCP
```

**Harness Engineering** é a prática de manter essa fórmula viva: versionar
spec, SDK, boilerplates e knowledge como artefatos de primeira classe — não
como "documentação de apoio".

> Sem harness, a IA improvisa. Com harness, a IA executa.

---

## MCP Light — O Que É e Por Que Existe

**MCP Light** é a arquitetura ideal para quem **não precisa de um MCP server**
para que a IA acesse banco, APIs, serviços externos e infraestrutura.

Em vez de expor cada recurso via servidor MCP, o Gofi expõe via **contexto
estruturado**:

| Recurso real        | MCP tradicional                  | MCP Light (Gofi)                                     |
|---------------------|----------------------------------|------------------------------------------------------|
| Banco de dados      | `db-mcp-server`                  | `sdk/go/sdk-docs/sqln.md` + boilerplate `repository` |
| HTTP / APIs         | `http-mcp-server`                | `sdk/go/sdk-docs/netx.md` + boilerplate `handler`    |
| Auth                | `auth-mcp-server`                | `sdk/go/sdk-docs/iam.md`                             |
| Mensageria          | `queue-mcp-server`               | `sdk/go/sdk-docs/msq.md`                             |
| Observabilidade     | `obs-mcp-server`                 | `sdk/go/sdk-docs/obs.md`                             |
| Erros recorrentes   | (não cobre)                      | `knowledge/*.md`                                     |
| Decisões anteriores | (não cobre)                      | `memory/contexts/*.md`                               |

**Resultado:** a IA acessa os mesmos recursos, mas:

- Zero processo extra rodando
- Zero latência de IPC
- Zero auth/secret management para o harness
- Versionado junto do código
- Auditável por `git diff`

> MCP Light **não substitui** MCP em todos os cenários. Substitui no cenário
> mais comum: a IA escrevendo código que será executado por humanos/CI, e não
> a IA *executando* recursos em runtime.

---

## Pipeline de Agents Especializados

Fluxo determinístico, cada agente com responsabilidade única. O **core**
(da ideia ao código auditado) é:

```
Requisito → gofi-pd → gofi-spec → gofi-eng → gofi-qa
            (PRD)      (SDD)       (backend)   (Auditoria)
                          │
                          └──────→ gofi-ui ──→ gofi-qa
                                   (frontend)
```

Cada agente é invocado como **skill** (`/gofi-pd`, `/gofi-spec`, …) e vive em
`skills/<nome>.md`. Todos são **genéricos e portáveis**: carregam apenas
metodologia; o que é específico do projeto vive em `specs/`, `memory/` e
`institutional/`.

### Core — do requisito ao código auditado

| Agente      | Responsabilidade                                  | NÃO faz                |
|-------------|---------------------------------------------------|------------------------|
| `gofi-pd`   | Product Discovery → PRD                            | Não escreve spec       |
| `gofi-spec` | Specification Architect → SDD                      | Não escreve código     |
| `gofi-eng`  | Context Engineer → backend 100% via SDK           | Não decide arquitetura |
| `gofi-ui`   | UI/UX Engineer → frontend (web/mobile) via DS     | Não decide arquitetura |
| `gofi-qa`   | Quality Auditor → aderência a spec + SDK          | Não altera código      |

### Apoio — entrega, documentação e estado

| Agente        | Responsabilidade                                          | NÃO faz                  |
|---------------|----------------------------------------------------------|--------------------------|
| `gofi-ops`    | Platform & Delivery → IaC, build e CI/CD a partir de spec | Não escolhe cloud/sizing |
| `gofi-doc`    | Documentation Generator → doc de API p/ front e QA        | Não edita código         |
| `gofi-status` | Índice de contextos derivado do `memory/contexts/*.md`    | Não escreve nada         |

A separação é o ponto: cada agente lê apenas o contexto da sua etapa, e o
output de um é input do próximo. **Sem sobreposição = sem inconsistência.**

---

## gofi-ui — Frontend com Design System (web e mobile)

O `gofi-ui` implementa a camada de apresentação a partir da spec (e do contrato
de API do `gofi-eng`). Ele **não cria componentes do zero**: consome um **Design
System publicado como dependência npm**, escolhido pela **superfície** do alvo —
não pelo framework cru:

| `ui.framework` (no `.gofi.yaml`) | Superfície | Design System (pacote npm) | Docs em                       |
|----------------------------------|------------|----------------------------|-------------------------------|
| `react` (`angular`/`vue`)        | **web**    | **`gofi-ui`**              | `sdk/web/gofi-ui/`            |
| `react-native` / `expo`          | **mobile** | **`gofi-ui-native`**       | `sdk/mobile/gofi-ui-native/` |

> A pasta de docs **é o nome do pacote**. Os `.md` do DS são a **especificação
> domínio-neutra** (tokens, componentes, patterns) que a lib implementa — não
> código a copiar. O agente faz `import { Button } from 'gofi-ui'`, nunca recria.

**Um projeto pode ter uma superfície ou as duas** (full-stack com backend Go +
web + mobile). Ambas bebem dos **mesmos tokens**
(`knowledge/ui/design-tokens.md`); muda só a **forma**:

| Tema   | `gofi-ui` (web)                          | `gofi-ui-native` (mobile)              |
|--------|-----------------------------------------|----------------------------------------|
| Stack  | React + TS + **Tailwind v4**            | React Native + TS                      |
| Tokens | utilitários (`bg-action`, `text-ink`)   | objeto TS (`makeTheme` + `useTheme()`) |
| Tema   | `[data-theme]` (light/dark)             | `useColorScheme()` + `<ThemeProvider>` |
| Navegação | rotas/URL                            | React Navigation (stack/tab) + safe-area |

Cada DS é organizado em **`foundations/`** (cor, tipografia, espaçamento,
acessibilidade…), **`components/`** (button, card, input, charts…) e
**`patterns/`** (app-shell, forms, states, page-templates…), com `gofi.md` como
ponto de entrada. **Nunca** se aplica o DS de uma superfície na outra.

---

## Layout do Repositório

Mistura conteúdo **genérico cross-AI/cross-language** (agents, knowledge
shared, templates) com **conteúdo específico por linguagem** sob `sdk/<lang>/`.
A pasta `ai/<provider>/` contém o que é específico do provedor de IA.
Em v1 só Claude Code é suportado.

```
.
├── skills/                   — um arquivo por agente (skill /gofi-*)
│   ├── gofi-pd.md            — Product Discovery
│   ├── gofi-spec.md          — Specification Architect
│   ├── gofi-eng.md           — Context Engineer (backend)
│   ├── gofi-ui.md            — UI/UX Engineer (web/mobile)
│   ├── gofi-qa.md            — Quality Auditor
│   ├── gofi-ops.md           — Platform & Delivery (IaC/CI/CD)
│   ├── gofi-doc.md           — Documentation Generator
│   └── gofi-status.md        — Índice de contextos
├── knowledge/
│   ├── shared/               — knowledge cross-agent/cross-language
│   ├── eng/                  — knowledge do gofi-eng
│   └── ui/                   — tokens, theming, ux-principles (gofi-ui)
├── specs-template/sdd-template.md
├── prd-template/prd-template.md
├── memory/
│   ├── project.md.tmpl       — semeado pelo `gofi init`
│   └── contexts/             — vazio inicialmente
├── ai/
│   └── claude/
│       ├── CLAUDE.md         — instruções raiz para Claude Code
│       └── README.md         — visão geral consumida no onboarding
├── cli/                      — código-fonte da CLI Go (módulo gofi-cli)
└── sdk/
    ├── go/                   — backend
    │   ├── boilerplates/     — model, repository, service, handler, …
    │   ├── sdk-docs/         — netx, sqln, iam, msq, obs, …
    │   └── knowledge/        — error-handling, pagination, value-objects, …
    ├── web/                  — frontend web
    │   ├── gofi-ui/          — Design System (foundations, components, patterns)
    │   ├── boilerplates/     — feature, page-route
    │   └── knowledge/        — structure, absolute-rules
    └── mobile/               — frontend mobile
        ├── gofi-ui-native/   — Design System (foundations, components, patterns)
        ├── boilerplates/     — screen, navigation
        └── knowledge/        — structure, absolute-rules
```

---

## Como a CLI Consome Este Repo

`gofi init` baixa este repo (um único tarball) e mescla em `<projeto>/.claude/`:

| Origem                                 | Destino                            |
|----------------------------------------|------------------------------------|
| `skills/<sel>.md` (selecionados)       | `.claude/skills/<sel>.md`          |
| `ai/claude/CLAUDE.md`                  | `.claude/CLAUDE.md`                |
| `specs-template/`                      | `.claude/specs-template/`          |
| `prd-template/`                        | `.claude/prd-template/`            |
| `knowledge/`                           | `.claude/knowledge/`               |
| `memory/project.md.tmpl` (rendered)    | `.claude/memory/project.md`        |
| `sdk/<surface>/boilerplates/`          | `.claude/sdk/<surface>/boilerplates/` |
| `sdk/<surface>/knowledge/`             | `.claude/sdk/<surface>/knowledge/`    |
| `sdk/go/sdk-docs/`                     | `.claude/sdk/go/sdk-docs/`            |
| `sdk/web/gofi-ui/` · `sdk/mobile/gofi-ui-native/` | `.claude/sdk/<surface>/<ds>/` (Design System) |

> `<surface>` é `go` (backend), `web` ou `mobile` — selecionada conforme
> `project.language` e o bloco `ui` do `.gofi.yaml`.

`gofi update` re-baixa o repo e refresca os mesmos paths **preservando**
`knowledge/{shared,pd,spec,eng,qa}/` (train-managed) e `memory/`
(project-state).

---

## Build a Partir do Código-Fonte

O código da CLI vive em [`cli/`](cli/) (módulo `github.com/joaoprofile/gofi-cli`).

```sh
cd cli
go build -o bin/gofi ./cmd
./bin/gofi h
```

Para um build local versionado:

```sh
cd cli
go build \
  -ldflags "
    -X github.com/joaoprofile/gofi-cli/internal/cli.Version=v0.0.0-dev
    -X github.com/joaoprofile/gofi-cli/internal/cli.Commit=$(git rev-parse --short HEAD)
    -X github.com/joaoprofile/gofi-cli/internal/cli.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  " \
  -o bin/gofi ./cmd
```

Releases são publicadas via GoReleaser quando uma tag `v*` é enviada
(ver [`.github/workflows/release.yml`](.github/workflows/release.yml) e
[`cli/.goreleaser.yaml`](cli/.goreleaser.yaml)).

---

## Conclusão

O Gofi Ecosystem transforma o desenvolvimento com IA em um processo
**determinístico, padronizado, auditável e evolutivo**.

Não é "gerar código com IA". É construir o **harness** onde:

- A arquitetura é respeitada por construção (DDA)
- Recursos reais são acessados sem MCP server (MCP Light)
- A qualidade é garantida por auditoria automática (gofi-qa)
- O conhecimento é acumulado entre execuções (knowledge + memory)

> **Harness Engineering: a disciplina de fazer a IA escrever código como o
> seu melhor engenheiro escreveria — todas as vezes.**
