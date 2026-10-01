# Pré-execução — descoberta do projeto e da topologia

## Pré-execução obrigatória — descoberta do projeto e da topologia

Antes de abrir handler, entender o projeto. É **aqui** que você aprende os
nomes reais (contextos, convenções de naming, casing de enum, envelope de
paginação, claims de auth) — a skill em si não os conhece. **Sempre, na
ordem:**

1. **`.gofi.yaml` (raiz)** — extrair `project.language` ({lang}),
   `project.name`, `project.path` e convenções de paths declaradas. Define
   a linguagem-alvo que orienta os passos seguintes.
2. **`AGENTS.md`** da raiz (já carregado) — mapa físico do projeto (onde ficam o código,
   as migrations, os binários) e as convenções de leitura.
3. **`.claude/memory/project.md`** — **qual binário monta o servidor HTTP**.
   Tipicamente há um único composition root HTTP (o serviço de API). É onde
   todas as rotas são registradas. Os demais serviços (workers, consumers,
   cron, adapters) não expõem HTTP — ignore para documentação de endpoint.
   Para o **mapa de contextos existentes**, rode `/gofi-status` ou liste
   `.claude/memory/contexts/`.
4. **`.claude/memory/contexts/{contexto}.md`** se já há handoff de fases
   anteriores — decisões de design (ADRs, presets, integrações) que afetam
   o contrato.
4b. **O grafo (`.gofi/index/code/`), quando existe** — é o caminho barato para a
   topologia: `gofi_graph_index.json` diz os escopos e **a pasta de cada um**
   (backend em `.`, cada superfície em `{nome}/`, SDK em `sdk/`); o
   `gofi_graph_report.md` **daquele escopo** dá pacotes e pontos centrais;
   `gofi find --in code "<termo> <termo>"` acha o símbolo quando você só tem a
   descrição; `gofi show {Handler}` mostra o que o
   handler chama (service/application, DTOs, erros) **sem abrir arquivo**, e
   `gofi path {Handler} {Repo}` traça o caminho até a
   persistência. Use-o para **escolher o que abrir**, não para substituir a
   leitura do handler — assinatura de rota, status code e tag de struct só o
   arquivo tem. **Nunca** abra `gofi_graph.json`. Protocolo:
   `.claude/expertise/harness-protocols/graph-retrieval.md`.
5. **`.claude/knowledge/INDEX.md` (núcleo ⬤ + só os módulos que a tarefa pede)** e **os módulos de `sdk/{lang}/knowledge/` que o `.claude/knowledge/INDEX.md` indicar**
   — **convenções reais do projeto**: naming de campo, casing de enum,
   envelope de paginação, formato de código de erro, shape do filtro
   dinâmico, formato de datas. **Tire as convenções daqui — não hardcode
   suposições.** Se a knowledge diz que enums são UPPER_SNAKE, que moeda é
   um campo `*Code`, que paginação é base-zero etc., é isso que vale.
6. **`.claude/sdk/{lang}/api/INDEX.md`** → só os arquivos dos pacotes que o
   endpoint usa (ou `gofi find --in sdk "<símbolo>"`) — API do SDK gerada do
   código (paginação, filtro dinâmico, erros). O *como usar* está em
   `.claude/sdk/{lang}/knowledge/` (`pagination.md`, `dynamic-filter.md`,
   `lookup-endpoints.md`, `error-handling.md`).
7. **`specs/{contexto}/sdd-{contexto}.md`** se existe — regras de negócio
   explícitas que estão no contrato mas podem não aparecer no código (ex.:
   ordem de prioridade, política de retry, garantias de idempotência).
8. **`prd/{contexto}/prd-{contexto}.md`** se existe — visão de produto,
   glossário, motivação de negócio. Útil para a "Visão geral" da doc e para
   responder dúvidas do dev sobre **por que** o endpoint se comporta assim.

Esses passos dão o mapa: linguagem-alvo, convenções de naming, onde estão
os contextos, qual binário expõe HTTP, qual a semântica de cada handler e
qual a motivação de produto.


## Layout convencional do projeto

A doc assume essa árvore (confirme contra o real lido nos passos de
pré-execução — nomes de pasta e prefixos variam por projeto):

```
services/
├── .migrations/                      # SQL up/down — schema canônico
├── domain/
│   └── {contexto}/                   # bounded context
│       ├── handler/                  # ← rotas HTTP do contexto
│       │   ├── {aggregate}_handler.go
│       │   └── middleware.go
│       ├── application/              # workflows (quando existe)
│       │   └── errors.go
│       ├── service/
│       │   ├── {aggregate}_service.go
│       │   └── errors.go             # ← códigos de erro REAIS
│       ├── repository/
│       │   └── {aggregate}_repository.go
│       └── model/
│           ├── entity.go
│           ├── {aggregate}_dto.go     # ← request/response shapes
│           └── *_constants.go
└── {api-service}/                    # composition root HTTP
    ├── main.go
    └── wire.go                       # ← onde os handlers são montados
```

**Regra de ouro:** **todo endpoint público está num `handler/` dentro de
um `domain/{contexto}/`**, e **toda rota está registrada no `wire.go` (ou
equivalente) do binário HTTP**. Se algo no `wire.go` chama
`xxxHandler.Handlers()` e o método existe no contexto, a rota é pública.

