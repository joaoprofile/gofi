# Regras de escrita da spec

## Regras de escrita da spec

### Nomenclatura — segue convenção da linguagem

Para Go: ver `.claude/sdk/go/knowledge/naming.md`. Em geral:
- Tabela SQL: singular, snake_case, em inglês — **nunca plural**
- Colunas SQL: snake_case
- Campos de entidade na linguagem-alvo: convenção da linguagem (PascalCase em Go, etc.)
- Endpoints: snake_case ou kebab-case no path, em inglês
- Erros: padrão `Err{Contexto}{Ação}` em inglês

### Conteúdo obrigatório

- RNs numeradas sequencialmente (RN-01, RN-02, ...)
- Campos de entidade com tipo da linguagem-alvo
- Filtros de listagem com comportamento exato (ILIKE, eq, range...)
- HTTP responses mapeados por caso (200, 201, 204, 400, 401, 403, 404, 409, 500)
- Validações por campo (required, email, min, max, oneof...)
- Ciclo de vida de status documentado se há transições
- Tabela SQL no singular
- Arquivos de teste sempre listados em §8 (Estrutura de Arquivos)
- **Migrations sempre como par `{N}_{nome}.up.sql` + `{N}_{nome}.down.sql`** em §3.4 (cabeçalhos dos blocos SQL: `-- 0001_create_x.up.sql`, **nunca** `.sql` puro) e em §8 (estrutura). `.sql` sem direção é silenciosamente ignorado pelo `golang-migrate` na hora de rodar — propagar isso na spec induz o `gofi-eng` ao mesmo erro. Ver `.claude/sdk/<lang>/knowledge/migrations.md` (regra absoluta #13)
- **IDs UUID em `tenant` e `user` (e em todas as FKs que apontem para eles)** — regra do projeto, **não** perguntar na elicitação. Aplica-se a qualquer linguagem-alvo. **Geração do UUID é responsabilidade da aplicação, sempre versão 7 (RFC 9562 — time-ordered)**: a coluna sai como `id UUID PRIMARY KEY` (sem `DEFAULT gen_random_uuid()`/`NEWID()`/`SYS_GUID()`), a migration **não** declara `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`, e a spec deve apontar (em §3 ou ADR) que o `service` gera o id v7 (Go: `uuid.NewV7()` do `github.com/google/uuid` ≥ v1.6.0) antes de `repo.Save(...)`. **Não** documentar `uuid.NewString()`/`uuid.New()` (esses retornam v4 — fragmentam índice B-tree e perdem ordenação temporal). Em consequência, contratos §0.1 não usam `INSERT ... RETURNING id`: `Save` recebe a entity já com `id` populado e devolve `error` (Go) — não `(*Entity, error)`. FKs: `tenant_id UUID REFERENCES tenant(id)` e `*_user_id UUID REFERENCES "user"(id)`. Em Go, modelar como `string` em entidade, DTOs, contratos de Repository/Service e path params. **Validators na borda usam `validate:"uuid"` (qualquer versão), não `uuid4` nem `uuid7`** — lock em versão quebra evolução do produtor e dispensa entrada legada. Outras entidades (pool, order, bettor, ledger…) seguem default sequencial (`BIGINT IDENTITY`). Justificativa completa, casos de exceção e checklist em `.claude/expertise/ddd-architecture/id-types.md`
- **Constraints UNIQUE com nome explícito** (`CONSTRAINT uq_<tabela>_<campos> UNIQUE (...)`), nunca auto-gerado pelo Postgres. Motivo: o repo discrimina conflito por `pgErr.Constraint` (SQLSTATE 23505), e nome auto-gerado muda se a ordem das colunas mudar. Detalhes em `.claude/sdk/go/knowledge/persistence-rules.md` §Conflito de UNIQUE.
- **Valor monetário, moeda e país — `services/common/money` é o pacote canônico**: quando o contexto lida com valor monetário, moeda ou país, a spec declara em §3/§4 que parse/format/arredondamento usam `services/common/money`, **não** inventa mapa país→moeda próprio, casas decimais por moeda nem heurística de parse. O catálogo (todos os países LatAm, `Currency{Code, Decimals, Symbol, DecimalSep, GroupSep}`) já vive no pacote. Sinalizar para o `gofi-eng`: (a) `money.ParseLoose` quando a **moeda é desconhecida no momento do parse** (import de planilha multi-país — moeda resolvida depois via tenancy); (b) `Currency.Parse`/`Format`/`Round`/`Truncate` quando a moeda **é conhecida** (engine, exibição); (c) país→moeda via `money.ByCountry`/`CodeForCountry` (fallback USD). Nos DTOs/contratos, campos são `currencyCode` (ISO 4217) e `countryCode` (ISO 3166-1 alpha-2) — nunca o struct `Currency` serializado nem `currency`/`country` crus.
- **§0.1 Contrato do Repository — métodos sempre na interface, nunca helpers de pacote**:
  toda operação que toca banco (single-table CRUD, aggregate methods, lookups,
  listagens) é declarada como **método da interface `{Contexto}Repository`** —
  a spec não menciona "helper `insertX` no pacote" nem "função utilitária
  `deleteYByZ`". Helpers privados que aparecem como sub-passos de um
  aggregate (ex.: `insertConfig` chamado dentro de `CreateAggregate`) são
  **decisão de implementação** — vivem como métodos privados do receiver
  (`func (r *xxxRepository) insertConfig(...)`) e **não aparecem** nem na
  interface nem na spec. Exceção implementacional: funções **puras** sem
  `ctx`/I/O (montadores de `[]any`, mapeamento entidade→args) podem ser
  funções privadas no pacote — a spec também não as documenta. Detalhes
  em `.claude/sdk/go/knowledge/repository-aggregate-pattern.md` §"Helpers
  de persistência são MÉTODOS do receiver" + regra absoluta #13 em
  `.claude/sdk/go/knowledge/absolute-rules.md`.
- **§8 Estrutura de Arquivos — service split CRUD vs Auth/IAM**: quando o
  contexto mistura CRUD de domínio com responsabilidades de auth/IAM
  (login, OAuth, refresh, logout, change/reset password, GetMe), a árvore
  em §8 deve mostrar `service/{contexto}_service.go` **e**
  `service/auth_service.go` (com `auth_service_test.go`), espelhando o
  split que o handler já tem (`{contexto}_handler.go` ↔ `auth_handler.go`).
  `errors.go` permanece único, sem `auth_errors.go`. A spec descreve uma
  única interface `{Contexto}Service` com TODOS os métodos (CRUD + auth) —
  o split é físico (arquivos), não contratual. Detalhes em
  `.claude/sdk/go/knowledge/structure.md` §"Split de service por
  responsabilidade".
- **Operações de Create — escolher entre devolver o recurso completo ou apenas confirmar**:
  - Default recomendado: `INSERT` simples sem `RETURNING`, repository devolve apenas `error`, service devolve `errs.AppError`, handler responde `201 Created` com corpo vazio (alinhado com `204 No Content` de Update/Delete).
  - Se o cliente **realmente** consome `id`/timestamps gerados pelo banco no body de `201`, ou se o service precisa do `id` para emitir evento/FK na mesma chamada, então a spec marca explicitamente "Create retorna recurso" — repo usa `RETURNING`, devolve `(*Entity, error)`.
  - Detalhes em `.claude/sdk/go/knowledge/persistence-rules.md` §Insert simples.
- **Diagramas de fluxo — PlantUML obrigatório**: §2 do SDD e qualquer
  outra seção que descreva fluxo (sequência de chamadas, ciclo de vida,
  evento/mensageria, orquestração cross-context) usa bloco fenced
  ` ```plantuml `. Mermaid, ASCII art, listas-como-diagrama e imagens
  externas são proibidos. Regra completa, catálogo de tipos e exemplos em
  `.claude/expertise/diagramming/conventions.md` (lido na pré-execução).
- **§8 Naming dos arquivos em `application/` — sufixo `_application.go`,
  NUNCA `_use_case.go`**: arquivos `{workflow}_application.go` com
  interface `{Workflow}Application`, struct privada `{workflow}Application`
  e constructor `New{Workflow}Application(...)`. A spec **não** descreve
  arquivos `evaluate_x_use_case.go` nem interfaces `XUseCase` — é
  convenção SDK pra todos os contextos do projeto.
- **§4/§8 Split decider/executor — DUAS bridges quando o contexto tem
  ambos**: o adapter por dimensão polimórfica (integração externa) implementa
  **duas bridges separadas** quando o contexto adota o pattern decider/executor
  do `.claude/expertise/event-driven/executor-pattern.md`:
  - `DecisionBridge` (puro, sem `ctx`/`error` de I/O) — usada pelo decider
    sobre estado local (lookup tables, filtros). Mora em
    `services/domain/{ctx}/bridge/decision_bridge.go`.
  - `ExecutionBridge` (com `ctx` + retry) — usada pelo executor para
    invocar o sistema externo. Mora em
    `services/domain/{ctx}/bridge/execution_bridge.go`.
  - Cada adapter implementa **as duas em arquivos separados**
    (`services/adapter/{dim}/{ctx}/decision_bridge.go` +
    `execution_bridge.go`). Se a spec menciona uma bridge única que
    "filtra E aplica", está errada — quebra snapshot consistente do
    decider. Spec declara as duas em §0.1 com cláusula explícita
    "DecisionBridge é puro, sem I/O".
- **§0.1 / §8 Loader pattern — snapshot consistente para motores de decisão**:
  quando o contexto tem motor de decisão (decider) que opera sobre snapshot
  de 3+ tabelas que precisam ser consistentes durante a avaliação, declarar
  em §8 a subpasta `services/domain/{ctx}/loader/` (irmão de `service/`,
  `repository/`, `application/`) com `contract.go` (interface `Loader` +
  struct `{Ctx}EvaluationContext`), `loader.go`, `queries.go`, `snapshot.go`,
  `errors.go`, `loader_test.go`. Em §0.1 declarar o contrato:
  `Loader.Load(ctx, entityID) (*EvaluationContext, errs.AppError)` +
  `Loader.ListBy{Dimension}(ctx, dim) ([]int64, errs.AppError)` +
  `Close() error`. Application/processor declaram dependência de `Loader`
  (não de N repositories). `ListBy{Dimension}` mora no Loader — **não**
  documentar subpasta `scheduler/repository/` paralela. Resolução de
  cascata de config (níveis hierárquicos) no SQL do Loader via `COALESCE`
  é aceitável e recomendada (replicação consciente com `XxxService.GetEffective`
  que continua sendo o caminho fora do motor). Padrão completo, decisão JOIN vs
  sub-query paralela, contrato de erros e anti-padrões em
  `.claude/sdk/go/knowledge/loader-pattern.md`.
- **§8 Processor scheduler-driven mora no DOMÍNIO, não no binário cron**:
  quando o contexto é acionado por cron periódico (em vez de consumer
  Kafka reativo), `gofi-spec` declara em §8 a subpasta
  `services/domain/{ctx}/scheduler/{processor,repository,model}/` —
  `Processor` implementa `scheduler.Processor` (de `services/common/scheduler`),
  repository de pendências lista entidades elegíveis paginadas. **Binário
  cron** do projeto é **só wiring** — importa `scheduler.NewRunner` +
  Processor do domínio e monta runner por dimensão. A spec **não** documenta
  `*_processor.go` no binário cron.
- **Types canônicos do envelope Kafka — substantivo (inbound/dado) vs gerúndio (outbound/processo)**:
  ao declarar `kafka.Type*` novo no envelope de eventos do projeto, escolher
  o nome conforme a **direção semântica** do evento (ver também
  `.claude/expertise/event-driven/kafka-type-naming.md`):
  - **Substantivo factual / singular** quando o type representa **dado inbound**
    (fato observado vindo de fora — sistema externo → nosso domínio):
    `Type{Snapshot}` onde `{Snapshot}` é o substantivo do dado observado.
    Consumidor típico: contexto de sincronização do projeto via adapter da
    dimensão polimórfica.
  - **Gerúndio / ação** quando o type representa **processo outbound**
    (comando do nosso domínio para fora — wb → externo): `Type{Acting}` onde
    `{Acting}` é o verbo no gerúndio. Consumidor típico: adapter do contexto
    que executa.
  - **Quando uma "dimensão" tem ambos os pipelines** (mesmo recurso com 1
    pipeline outbound + 1 inbound), declarar **dois types distintos** no
    envelope — **não** usar 1 type com `source` discriminador. Motivo:
    consumer downstream filtra por `type` (consumer group próprio), não por
    `source`; types separados permitem consumer groups distintos sem
    competição por mensagens e sem branching de dispatcher no consumer.
  - **Anti-padrão**: nomear o type igual ao **contexto de domínio** que o
    consome (e.g. `Type{Ctx}` consumido por `services/domain/{ctx}/`) **e**
    também usar o mesmo nome para evento inbound — vaza ambiguidade para
    o consumer. Quando confundir, perguntar: "esse evento é um dado factual
    que chegou de fora ou é um comando que o domínio emitiu?".

### Manifesto do Serviço (§0)

Toda spec abre com a seção §0 — campos derivados das respostas da Fase 1.
O formato exato vem do `templates/sdd-template.md`. Para Go, os
nomes de path (`pathService`, `pathCmd`, `pathContext`) seguem
`.claude/sdk/go/knowledge/structure.md`.

> **Bootstrap do binário (`pathCmd`):** quando o serviço carrega providers
> (JWT, Redis session, OAuth), adapters de SDK externo (IAM tenant/RBAC) ou workers,
> a spec **não** lista os arquivos de bootstrap em §8 (eles são cmd-level,
> não context-level). O `gofi-eng` decide automaticamente se split em
> `config.go`/`wire.go`/`iam.go`/`config_test.go` (composition root com
> `gofi.New(name).With(componentes...).Build()`) se aplica — ver
> `.claude/sdk/go/knowledge/service-bootstrap.md` e `gofi-orchestrator.md`. A spec só precisa
> sinalizar que o contexto envolve auth/IAM (Fase 4) — o split do cmd
> decorre disso.

> **`pathService` = `project.path` do `.gofi.yaml`** (raiz do módulo Go) e
> **`pathCmd` = `pathService/{projectName}/`** (subdiretório homônimo ao
> `project.name`, abriga o `main.go`). Por exemplo, `path: backend` +
> `name: web-api` ⇒ `pathService = ./backend/`,
> `pathCmd = ./backend/web-api/`, `pathContext = ./backend/domain/{contexto}/`.
> O `module` declarado no `go.mod` **não** inclui `pathService` nem
> `projectName` — imports são do tipo `{module}/domain/{contexto}/...`,
> sem o `web-api/` no caminho.

