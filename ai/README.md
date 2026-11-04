# Gofi — Onboarding

> Este projeto foi gerado pelo `gofi init`. A pasta de agentes que você tem
> agora é o **harness** — skills, especialidades, SDK, boilerplates, knowledge
> e memória — sobre o qual o agente de código vai operar. O nome da pasta é o
> que o host escolhido no `init` descobre: `.claude/` para o Claude Code,
> `.agents/` para Codex e Copilot. Os exemplos abaixo usam `.claude/`.

Para a tese arquitetural completa (DDA, Harness Engineering, MCP Light), veja
o [README do gofi-agents](../../../readme.md). Este documento foca no que
você recebeu **dentro do projeto** e em como usá-lo.

---

## O Que É Este Harness

```
DDA  =  SDD                  ← o contrato antes do código: spec viva + mudanças (decisão 0003)
      + SDK                  ← única fonte de verdade para implementação
      + Padrões + SOLID + DDD ← invariantes de design
      + Boilerplates         ← estrutura por camada
      + Knowledge            ← regras escritas a partir de erros passados
      + Context (MCP Light)  ← acesso a DB/API/infra via SDK + docs, sem MCP server
```

**Resultado prático:** quando você roda uma skill (`/gofi-spec`, `/gofi-eng`, …),
ela lê automaticamente todo esse contexto antes de produzir qualquer output.
Sem improviso, sem alucinação de convenção, sem repetição de erros já
corrigidos.

---

## Pipeline das Skills

```
problema bruto  →  /gofi-pd  →  /gofi-spec  →  aprovação  →  /gofi-eng  →  /gofi-qa  →  arquivamento
                   (PRD)        (mudança)     (a pessoa)     (código)      (auditoria)   (spec viva)
```

**O contrato.** `specs/{contexto}/spec.md` é a verdade viva de cada contexto —
requisitos `REQ-NN` com DEVE e cenários DADO/QUANDO/ENTÃO, sem tecnologia — e
não se edita à mão. Todo trabalho é uma **mudança** em `specs/{contexto}/changes/{id}/`:
proposta, um delta por contexto (ADICIONADO / MODIFICADO / REMOVIDO) e o design
no template do pack da linguagem. A pessoa aprova (`gofi change approve`), o
loop implementa e audita por cenário, e `gofi change archive` aplica os deltas
às specs vivas — sem modelo. Contexto novo é a mudança que só adiciona.

- **Camada de apresentação:** `/gofi-ui` após a spec (e o contrato do `gofi-eng`).
- **Plataforma / infra:** `/gofi-spec` (spec de infra) → `/gofi-ops`.
- **Doc de contrato:** abrir o handler no IDE → `/gofi-doc`.
- **Panorama do projeto:** `/gofi-status` a qualquer momento.

Cada skill tem responsabilidade única e **não invade** o território da
próxima. O canal de handoff é a memória em
[`.claude/memory/contexts/{contexto}.md`](.claude/memory/contexts/).

---

## Doutrina das Skills

> **Skill carrega só especialidade transferível.** Cada skill é um especialista
> **genérico e portável**: leva metodologia e técnica que serviriam, sem mudar
> uma palavra, a outro projeto com o mesmo SDK. **Nada** de produto/empresa/
> instituição entra na skill (nomes de entidade, roles, module paths, endpoints,
> valores de negócio). Trocar de projeto **não** muda a skill.
>
> **Conhecimento específico mora FORA da skill** — em `.gofi.yaml`, `specs/`,
> `.claude/memory/` e `.claude/institutional/`. Padrão técnico genérico vive em
> `.claude/expertise/` e `.claude/sdk/<lang>/`, sempre **domínio-neutro**
> (placeholders `{contexto}`, `<module>`, `RoleA`, `entity`).
>
> **Teste de pertencimento:** *serviria, sem mudar uma palavra, a outro projeto
> com o mesmo SDK? → skill/knowledge. Só vale aqui? → spec/memória/institucional.*

Detalhe completo no [`AGENTS.md`](../AGENTS.md) da raiz do projeto.

---

## As 8 Skills

| Skill          | Função                                                                                     | NÃO faz                |
|----------------|--------------------------------------------------------------------------------------------|------------------------|
| `/gofi-pd`     | Product Discovery — transforma problema bruto em PRD navegável                              | não escreve spec       |
| `/gofi-spec`   | Specification Architect — escreve mudanças (proposta, deltas, design) e a baseline da spec viva (inclui infra) | não escreve código nem aprova |
| `/gofi-eng`    | Context Engineer — implementa o contexto Go (model → repository → service → handler)        | não decide arquitetura |
| `/gofi-ui`     | Context UI/UX — implementa a camada de apresentação de uma mudança aprovada                | não toca backend       |
| `/gofi-ops`    | Platform & Delivery — DevOps (IaC, build, CI/CD) a partir de uma mudança de infra aprovada | não escolhe cloud      |
| `/gofi-qa`     | Quality Auditor — audita a mudança cenário a cenário, contra o design, o SDK e as regras    | não reescreve código   |
| `/gofi-doc`    | Documentation Generator — gera doc de contrato a partir dos handlers Go                     | read-only sobre código |
| `/gofi-status` | Índice de Contextos — monta sob demanda o panorama lendo o frontmatter dos `contexts/*.md` | não edita memória      |

### [`/gofi-pd`](.claude/skills/gofi-pd/SKILL.md) — Product Discovery

Transforma um problema de negócio em PRD navegável: objetivos, personas,
jobs-to-be-done, métricas de sucesso e escopo.

### [`/gofi-spec`](.claude/skills/gofi-spec/SKILL.md) — Specification Architect

Escreve o contrato como **mudanças** (decisão 0003).

- Elicitação estruturada; cada fase alimenta um documento da mudança
- Requisitos `REQ-NN` (DEVE / NÃO DEVE) com cenários DADO/QUANDO/ENTÃO — o quê, sem tecnologia
- Design da mudança no template do pack: dados, contratos, erros, mapa de requisitos
- Baseline da spec viva de um contexto que já roda, pelo código, pelo grafo e pelos testes
- Produz também as mudanças de **infra/plataforma** consumidas pelo `gofi-ops`
- **Não escreve código e não aprova** — `gofi spec check` em zero erros, e a pessoa aprova

### [`/gofi-eng`](.claude/skills/gofi-eng/SKILL.md) — Context Engineer

Implementa uma mudança aprovada, em camadas, na linguagem do `.gofi.yaml`.

- Implementa model, repository, service, handler e testes
- Usa estritamente os módulos do SDK (`sqln`, `netx`, `base`, `obs`, `iam`, `msq`)
- Aplica boilerplates como esqueleto de cada camada
- Registra arquivos criados e decisões na memória do contexto
- Atualiza a própria spec (rastreabilidade, histórico, divergências)

### [`/gofi-ui`](.claude/skills/gofi-ui/SKILL.md) — Context UI/UX

Implementa a camada de apresentação (pages, features, components, layouts,
hooks) a partir da spec aprovada e — quando existir — do contrato do `gofi-eng`.

- Framework lido de `ui.framework` no `.gofi.yaml` (React + TS primeiro; Angular/Vue em expansão)
- UX **não é decoração** — toda tela passa pelos cinco princípios de
  [`expertise/ui-design/ux-principles.md`](.claude/expertise/ui-design/ux-principles.md)
- Não escreve fora do escopo da spec; pergunta antes de codificar quando falta contexto

### [`/gofi-ops`](.claude/skills/gofi-ops/SKILL.md) — Platform & Delivery

Engenheiro DevOps: infraestrutura como código, empacotamento de artefato e
pipelines de CI/CD a partir da **spec de infra aprovada**.

- Stack lida do bloco `ops:` no `.gofi.yaml` (cloud, IaC, runtime, CI/CD, registry)
- Domina Terraform, OCI, build Go, CI/CD (Azure DevOps / GitHub Actions) como competência
- **Não escolhe** cloud, topologia ou sizing por conta própria — vem do `ops:` + spec
- Toda mudança passa por `plan`/`diff` aprovado **antes** de `apply`/`deploy`

### [`/gofi-qa`](.claude/skills/gofi-qa/SKILL.md) — Quality Auditor

Audita implementação contra spec, SDK e regras absolutas.

- Audita uso correto da API do SDK (fonte: `sdk/<lang>/api/` + `sdk/<lang>/knowledge/`)
- Verifica aderência a spec, boilerplates e knowledge acumulada
- Checa regras absolutas (erros, logging, camadas, mocks, IAM, netx)
- **Não reescreve código** — reporta e sugere correções específicas

### [`/gofi-doc`](.claude/skills/gofi-doc/SKILL.md) — Documentation Generator

Gera doc de contrato de API derivada do código real, para **dois públicos**:
engenheiro de frontend (cliente) e QA tester (plano de teste).

- **Read-only sobre código** — nunca edita, refatora ou sugere mudança de implementação
- Nunca inventa comportamento; ambiguidades vão para "Armadilhas conhecidas" com tag `[inferido]`

### [`/gofi-status`](.claude/skills/gofi-status/SKILL.md) — Índice de Contextos

Monta **sob demanda** o panorama de todos os contextos (Implementados / Spec /
PRD), derivado do frontmatter de cada `contexts/*.md`.

- O índice **não é commitado** — sem alvo de escrita comum → zero conflito de git
- **Read-only** — reporta inconsistências (frontmatter faltando, path inexistente), nunca corrige

---

## Estrutura da pasta de agentes

```
.claude/
├── skills/                         — uma PASTA por skill (slash /gofi-*), cada
│   │                                 uma com SKILL.md + frontmatter `name`/`description`
│   ├── gofi-pd/SKILL.md    gofi-spec/SKILL.md   gofi-eng/SKILL.md   gofi-ui/SKILL.md
│   └── gofi-ops/SKILL.md   gofi-qa/SKILL.md     gofi-doc/SKILL.md   gofi-status/SKILL.md
├── sdk/<lang>/
│   ├── boilerplates/               — esqueletos por camada (linguagem-específicos)
│   ├── api/                        — referência do SDK gofi, gerada do código (MCP Light)
│   └── knowledge/                  — knowledge específico da linguagem
├── expertise/<pack>/               — especialidades: PACK.md (contrato) + seções
│   └── harness-protocols/ ddd-architecture/ event-driven/ diagramming/ ui-design/ platform-delivery/ code-review/
├── knowledge/                      — aprendizado do time (o gofi nunca reescreve)
│   ├── shared/                     — vale para todos os papéis
│   └── {eng,ui,...}/               — de um papel (protocolo de aprendizado)
├── institutional/{project.name}/   — RAG de negócio (INDEX.md + chunks) — espelho do repo da empresa (pull-only) ou mantido à mão
├── templates/                      — spec, proposal, delta, design (neutro), tasks e prd (*-template.md)
└── memory/
    ├── project.md                  — visão global (serviços + convenções)
    └── contexts/{contexto}.md      — estado por-contexto (frontmatter) + handoff entre skills
```

Onde `<lang>` corresponde a `project.language` em [`.gofi.yaml`](.gofi.yaml).
Em v1 o SDK cobre **`go`** (backend) e o front (`/gofi-ui`): web pelo DS
**`gofi-ui`** (`sdk/web/gofi-ui/`) e mobile pelo DS **`gofi-ui-native`**
(`sdk/mobile/gofi-ui-native/`).

> **Modelo de duas camadas** (v2.5+): `.claude/` é a camada de decisão da IA
> — markdown destilado consumido pelas skills. `.gofi/gofi-sdk-<lang>/` é a
> camada de execução do toolchain (código real do SDK, importável pelo módulo
> do projeto via `go.work`); as skills normalmente NÃO leem daí. Se uma skill
> precisou abrir código real para decidir, é gap de curadoria — registrar em
> `.claude/knowledge/` e oferecer a correção ao upstream.

---

## Três camadas de atualização (quem atualiza o quê)

Cada tipo de conteúdo tem **uma** fonte e **um** comando — nunca se cruzam:

| Camada | Conteúdo | Fonte | Comando | Versionado no projeto |
|--------|----------|-------|---------|-----------------------|
| **Técnica / compartilhada** | skills, especialidades (`expertise/`), SDK, referência de API, boilerplates, templates, AGENTS.md (menos o bloco do projeto) | `sources.agents` (+ `sources.sdk`) | `gofi update` (puxa sempre o último) | sim |
| **Institucional (negócio)** | `.claude/institutional/{project.name}/` | `sources.institutional` (repo da empresa, multi-produto) | `gofi update institutional` (**substitui a pasta por completo**) | sim (snapshot) |
| **Do projeto** | `prd/`, `specs/`, `memory/contexts/`, `knowledge/`, `lexicon/` | git do próprio projeto | nenhum (a CLI **nunca** toca) | sim |

- `gofi update` **preserva** `knowledge/` e `memory/` e **não toca** `institutional/`.
- **Sem `sources.institutional`:** a pasta institucional nasce de um starter local
  e é editada **à mão** no git do projeto — não há de onde puxar.
- **Com `sources.institutional`:** a pasta vira **espelho pull-only**; edições de
  negócio vão para o repo da empresa, não para o projeto.

---

## SDK Reference — `.claude/sdk/<lang>/api/`

Referência de API do SDK gofi, **gerada do código-fonte** do checkout que o
projeto fixou (`sources.sdk`) — um arquivo por pacote, uma seção por símbolo
exportado, lista em `api/INDEX.md` e exemplos em `api/examples.md`. Não se
edita à mão: regenera com `gofi update sdk`, então não defasa da versão do
código. **Esta é a "MCP Light":** a skill acessa DB, HTTP, auth, mensageria e
observabilidade pelo SDK documentado aqui — sem precisar de MCP server.

O *como usar* (regras, armadilhas, padrões) fica curado em
`.claude/sdk/<lang>/knowledge/` — `gofi-orchestrator`, `persistence-rules`,
`http-server`, `iam`, `messaging-msq`, `observability-otel`, `configuration`, …
A `gofi index check` aponta citação de API que o SDK fixado não tem.

---

## Boilerplates — `.claude/sdk/<lang>/boilerplates/`

Esqueletos prontos por camada de um contexto gofi. Consumidos por `gofi-eng`
como referência de forma e por `gofi-qa` como referência de correção.

| Arquivo              | Conteúdo                                                          |
|----------------------|-------------------------------------------------------------------|
| `main.md`            | `main.go` do serviço: bootstrap, config, registro de rotas        |
| `model.md`           | Entidade com tags db + DTOs com tags validate e `Validate()`      |
| `repository.md`      | Interface + SQL + implementação no mesmo arquivo                  |
| `service.md`         | Regra de negócio, `errs.AppError`, orquestração                   |
| `handler.md`         | Parse → service → `netx.Response` / `netx.Error`                  |
| `handler-test.md`    | Padrão de teste de handler com mock handcrafted                   |

---

## Knowledge — Onde Vive Cada Coisa

Quatro camadas, dois donos:

| Camada                                | Escopo                         | Quem mantém                  |
|---------------------------------------|--------------------------------|------------------------------|
| `.claude/expertise/<pack>/`           | especialidade, vários papéis   | gofi (`gofi update`)         |
| `.claude/sdk/<lang>/knowledge/`       | uso do SDK da linguagem        | gofi (`gofi update sdk`)     |
| `.claude/sdk/<lang>/api/`             | API do SDK (gerada do código)  | gofi (`gofi update sdk`)     |
| `.claude/knowledge/{shared,<papel>}/` | aprendizado do time            | o projeto — o gofi não toca  |

**Especialidades (`expertise/`)** — um pack é um domínio coerente, escrito uma
vez e usado por todo papel que precisa dele. O `PACK.md` declara quando ele se
aplica (imports, símbolos, intenções, entidades, paths) e a quais papéis serve;
`harness-protocols` (retrieval, memória, versionamento, aprendizado) vale para
toda tarefa. Packs atuais: `harness-protocols`, `ddd-architecture`,
`event-driven`, `diagramming`, `ui-design`, `platform-delivery`, `code-review`.

**Conteúdo típico** em `sdk/go/knowledge/` (Go): `absolute-rules`, `layers`,
`naming`, `structure`, `gofi-orchestrator`, `configuration`,
`persistence-rules`, `transactions`, `dynamic-filter`, `error-handling`,
`http-server`, `iam`, `messaging-msq`, `consumer-bootstrap`,
`worker-bootstrap`, `observability-otel`, `logging`, `qa-checklist`, e demais
padrões do SDK.

**Aprendizado do time (`knowledge/`)** — correção, ensino ou validação do
usuário que vale para este projeto. Vence `expertise/` e `sdk/` na busca e,
quando corrige uma regra do gofi, declara `overrides:` no frontmatter. Uma
correção que serviria a qualquer projeto é oferecida ao upstream.

> **Contexto institucional (RAG)** — quando uma skill precisa de negócio além
> da spec, lê `.claude/institutional/{project.name}/INDEX.md` e **só os chunks
> relevantes**, nunca a pasta inteira. É conteúdo **específico** do produto/
> empresa — vive fora das skills por definição.

---

## Memória — `.claude/memory/`

Canal de comunicação entre skills e entre sessões.

- **`project.md`** — visão global **específica do projeto**: serviços/binários e
  convenções deliberadas. Lida por **todas** as skills antes de executar. Padrão
  técnico genérico do SDK (stack, env vars) vive em `.claude/sdk/<lang>/`, não
  aqui. **Estado por-contexto NÃO vai aqui** (evita conflito de git entre devs).
- **`contexts/{contexto}.md`** — estado e handoff por contexto. **Frontmatter**
  (`status`, `versao_*`, `atualizado`) é o **único** lugar de estado por-contexto;
  o corpo guarda o histórico. Cada skill escreve sua parte ao concluir:
  - `gofi-pd`   → decisões de produto
  - `gofi-spec` → decisões de infra/domínio
  - `gofi-eng`  → arquivos criados + decisões não-óbvias
  - `gofi-ui`   → telas/componentes implementados
  - `gofi-ops`  → recursos provisionados + pipelines
  - `gofi-qa`   → score + pendências (`auditado` / `reprovado`)

O índice global (Implementados / Spec / PRD) é **gerado sob demanda** por
`/gofi-status`, lendo o frontmatter de cada `contexts/*.md`. Protocolo completo
em [`expertise/harness-protocols/memory.md`](.claude/expertise/harness-protocols/memory.md).

---

## Configuração — `.gofi.yaml`

Arquivo na raiz do projeto. Cada skill lê **antes de qualquer coisa** para
descobrir linguagem-alvo, nome do projeto e demais configurações.

```yaml
project:
  name: my-service
  language: go        # determina sdk/<lang>/ lido por gofi-eng/qa
ui:
  framework: react    # determina o stack de /gofi-ui
ops:
  cloud: oci          # determina a stack de /gofi-ops (IaC, CI/CD, registry)
sources:
  agents: github.com/org/gofi@main          # base técnica → gofi update
  institutional: github.com/org/base@main   # base de negócio → gofi update institutional (opcional)
# ... demais campos consumidos pelas skills
```

`project.language` determina os paths de SDK/boilerplates do backend
(`sdk/go/...`); `ui.framework` direciona o `/gofi-ui`; o bloco `ops:` direciona
o `/gofi-ops`.

---

## AGENTS.md — Instruções Raiz

[`AGENTS.md`](../AGENTS.md), na raiz do projeto, é o documento mestre lido pelo agente — Claude Code, Codex, Copilot, Cursor —
antes de qualquer skill. Define:

- A **doutrina das skills** (portável vs específico) e o mapa de paths físicos
- A convenção de leitura comum das skills (ordem de pré-execução)
- O protocolo de persistência de estado — o que cada skill escreve ao concluir
- O protocolo de aprendizado contínuo — como correções do usuário propagam para
  `sdk/<lang>/knowledge/`, `sdk/<lang>/boilerplates/`, `knowledge/` e `skills/`

---

## Estrutura de um Contexto Gerado

Output típico do pipeline no serviço-alvo (backend):

```
{pathService}/                     — ex.: ./src/
  go.mod
  .migrations/
  {projectName}/main.go            — ex.: ./src/web-api/main.go
  domain/
    {contexto}/
      model/        entity.go, dto.go
      repository/   {contexto}_repository.go      ← interface + SQL + impl (arquivo único)
      service/      errors.go, {contexto}_service.go, {contexto}_service_test.go
      handler/      {contexto}_handler.go, {contexto}_handler_test.go
      adapter/      — apenas se o contexto implementa portas de SDKs externos (IAM, etc.)
```

---

## Como Usar

1. **Descoberta** — `/gofi-pd` transforma problema em PRD em `prd/{contexto}/`
2. **Especificação** — `/gofi-spec` escreve a mudança em `specs/{contexto}/changes/{id}/`; `gofi spec check {id}`; a pessoa aprova com `gofi change approve {id}`
3. **Implementação (backend)** — `/gofi-eng` implementa a mudança aprovada
4. **Apresentação** — `/gofi-ui` implementa a UI da mudança, sobre o contrato do backend
5. **Plataforma** — `/gofi-ops` provisiona IaC + pipelines de uma mudança de infra
6. **Auditoria** — `/gofi-qa` valida cenário a cenário, contra o design, o SDK e o knowledge; aprovado limpo, `gofi change archive {id}` aplica a mudança às specs vivas
7. **Documentação** — `/gofi-doc` gera doc de contrato a partir dos handlers
8. **Panorama** — `/gofi-status` mostra o estado de todos os contextos
9. **Iteração** — correções do usuário alimentam `knowledge/`, pelo protocolo de aprendizado

> Você pode entrar pelo passo 2 se já tiver clareza de produto, ou pelo passo 3
> se já tiver uma mudança aprovada. Um projeto com specs no formato antigo
> (`sdd-{contexto}.md`) migra com `gofi spec migrate` (ver `/gofi-migrate`). `/gofi-qa`, `/gofi-doc` e `/gofi-status` rodam
> em qualquer momento sobre contextos existentes.

---

## Aprendizado Contínuo

Quando você corrigir, ensinar ou validar algo não-óbvio, **todas as skills
aprendem juntas** — não apenas a que recebeu a correção. O protocolo está em
[`expertise/harness-protocols/learning.md`](.claude/expertise/harness-protocols/learning.md):
o agente grava a lição em `knowledge/` no momento da correção.

Evolução típica:

- Novo módulo/API no SDK → `gofi update sdk` regenera `sdk/<lang>/api/`; o uso curado vai em `sdk/<lang>/knowledge/`
- Novo padrão de projeto → atualiza `sdk/<lang>/boilerplates/` e `sdk/<lang>/knowledge/`
- Correção do usuário revela regra não-documentada → registra em
  `knowledge/{shared,<papel>}/`; se serviria a qualquer projeto, oferece ao upstream
- Novo contexto implementado → memória cresce em `memory/contexts/`

---

## Para Saber Mais

- Tese arquitetural (DDA, Harness Engineering, MCP Light): [`gofi-agents/readme.md`](../../../readme.md)
- Instruções raiz das skills: [`AGENTS.md`](../AGENTS.md) (raiz do projeto)
- Especialidades: [`.claude/expertise/`](.claude/expertise/)
