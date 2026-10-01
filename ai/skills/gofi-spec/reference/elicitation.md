# Processo de elicitação

## Processo de Elicitação

### Fase 1 — Identificação do serviço (sempre primeiro)

Antes de modelar qualquer domínio, identifique **onde** o contexto vive.

**Localização e identidade:**
- Nome do serviço dentro do monorepo (ex: `auth-service`, `billing-service`)
- Module path da linguagem (ex: `github.com/org/projeto/backend/auth-service`)
- Serviço já existe ou é criado do zero?
- Se existe: caminho atual; se novo: pasta pai

**Infraestrutura:**
- Porta HTTP (ex: `:8080`)
- Banco (PostgreSQL, MySQL, SQL Server, Oracle)
- Usa Redis? Para que (cache, sessão, rate-limit)?
- Já existe wiring/bootstrap (`main.go` ou equivalente) configurado?

**Contextos existentes:**
- Outros contextos no mesmo serviço? (lista para o gofi-eng registrar rotas junto)
- Middleware de autenticação configurado? Grupos de rotas (`/api/v1`, `/internal`)?

**Prefixo de rotas:**
- Prefixo base deste contexto (ex: `/api/v1`)
- Rotas de auth em grupo separado?

### Fase 2 — Modelagem DDD do domínio

Use `expertise/ddd-architecture/ddd-principles.md` como guia. Aprofunde cada item:

**Identidade do contexto:**
- Nome do contexto em inglês, singular (ex: `order`, `product`, `invoice`)
- Linguagem ubíqua: termos do domínio (em pt) que devem ser consistentes

**Agregado e entidades:**
- Entidade raiz do agregado
- Entidades filhas / VOs aninhados (ex: `OrderItem` em `Order`, `Money{Amount, Currency}`)
- Atributos da raiz: nome (en), tipo, obrigatoriedade, validações
- Relações com outros contextos (FK + cardinalidade)

**Value Objects aninhados:**
- Quais grupos de campos formam VOs?
- Persistência: colunas separadas (mapper expande recursivamente) ou coluna única JSON (implementa Scanner/Valuer)?
- Detalhes language-specific em `.claude/sdk/<lang>/knowledge/value-objects.md`

**Eventos de domínio:**
- O que acontece de relevante? (ex: "pedido aprovado", "fatura vencida")
- Algum evento notifica outros contextos?

**Invariantes:**
- O que **nunca pode acontecer**? (ex: "email duplicado por tenant", "saldo negativo")
- Há transições de estado? Mapeie o ciclo de vida completo

### Fase 3 — Operações e API

- Operações: CRUD, especiais (aprovar, cancelar, exportar...)
- **Para cada operação de listagem, marcar explicitamente: simples ou paginada?**
  - **Simples** — lista bounded por natureza (filhos de um agregado, papéis
    de um usuário, lojas de um tenant). API **não** expõe `page`/`limit`.
    Implementação Go: `sqln.FindFromCriteria[T](...).List()` devolvendo
    `([]T, error)`. Sem `WithPage`, sem envelope `Page`. Detalhes em
    `.claude/sdk/<lang>/knowledge/pagination.md`.
  - **Paginada** — lista unbounded ou exposta com `page`/`limit` na API.
    Definir: ordenação default, limite default, filtros aceitos.
- Filtro estático (query params fixos) **ou** dinâmico (`/schemas` + `/query`)?
  - Dinâmico exige: campos filtráveis (nome de API → `Column` SQL, Label, FilterType), ordenáveis (`Sortable`), operadores permitidos (`Ops`), filtro default — campo/operador/sort fora do mapping é rejeitado (400) pelo SDK
  - Filtro dinâmico **implica paginada** (`/query` retorna `sqln.Page[T]`)
  - Detalhes em `.claude/sdk/<lang>/knowledge/dynamic-filter.md`
- **Para cada campo do `{Ctx}FilterMapping`, classificar (metadados de UI do `sqln.FilterField`):**
  - **text/number/boolean** — busca por substring/range/flag. `SearchType` e `Content` ficam vazios.
  - **search-multiple** — front renderiza multi-select. Operadores `IN`/`=`. Casos:
    - `SearchType: "embedded"` + `Content: enums.XxxStatusMap` — enum estático
      inline na resposta de `POST /{ctx}s/schemas`. Front consome direto, sem round-trip.
    - `SearchType: "v1/<path>"` (sem `/` inicial) — path da API que retorna os valores
      dinamicamente (lookup cross-context com DB, cardinalidade alta, paginação).
      `Content` fica `nil`. Endpoint é propriedade do contexto-dono — aqui só guardamos a referência.
  - **search-single** — idem `search-multiple`, mas front renderiza select/radio (uma escolha).
    Decisão `multiple` vs `single` é UX/produto, não domínio.
  - **Não existe mais endpoint dedicado `GET /{ctx}/status`** — o front lê
    `schema[campo].content` direto da resposta de `POST /{ctx}s/schemas`. Spec **não** declara `/status`.
  - Para cada enum embedded, a spec aponta a **constante de origem**
    (canônico: `services/common/enums/{topico}.go`, pacote único `enums` com
    prefixo nas constantes; aceito: `services/common/{contexto}/` se o repo
    já usa pacote por contexto).
  - Detalhes, decisão `multiple` vs `single`, anti-padrões e checklist em
    `.claude/sdk/<lang>/knowledge/lookup-endpoints.md`.
- Regras de acesso por operação (autenticado, RBAC, owner-only, admin)

### Fase 4 — Arquitetura e infraestrutura

**Cache:**
- Precisa? Backend (Redis), estratégia de invalidação (TTL, evento, escrita)
- Dados que nunca devem ser cacheados (sensíveis, tempo real)
- **Para cada leitura cacheada, classificar:**
  - **Single-query** — leitura sai inteira de uma chamada SDK
    (`FindFromCriteria` ou `FindWithFilter`). Implementação Go: cache
    inline via `.WithCache(sqln.NewCache[T](...))`.
  - **Composite/DTO** — resultado é DTO montado por **múltiplas** queries
    ou lógica adicional (loops, merges). Implementação Go: cache manual
    via `cache.UniqueResult` no início + `cache.Set` no fim.
  - Detalhes em `.claude/sdk/<lang>/knowledge/cache-layer.md`

**Mensageria:**
- Publica eventos? Quais e em qual tópico/fila?
- Consome eventos de outros contextos?
- Broker (Kafka, RabbitMQ, SQS, Redis Pub/Sub)
- **Para cada consumer**, declarar: tópico, consumer group e — se for decisão de domínio — concorrência default (`{Topic}Concurrency`). Caso contrário, eng escolhe default razoável. A spec **não** detalha wiring do `ConsumerManager` (criação, `Dispatcher(n)`, `Close`): é responsabilidade do `gofi-eng` montar o wrapper `{Topic}Consumer` em `pathCmd` que possui o `*msq.ConsumerManager` internamente (1 wrapper = 1 manager). Padrão completo em `.claude/sdk/go/knowledge/consumer-bootstrap.md`.

**Scheduler / cron (quando o contexto tem job periódico):**
- Acionamento **por intervalo** (a cada N) ou **horário fixo diário** (sweep "noturno")? Para horário fixo, a spec declara: hora/minuto (escalonados por dimensão quando há N jobs do mesmo tipo) + **fuso de negócio explícito** (não o tz do container/UTC). `gofi-eng` embute `_ "time/tzdata"` no binário cron (senão `LoadLocation` panica no boot). Padrão em `.claude/sdk/go/knowledge/worker-bootstrap.md` §"Cron com horário fixo".

**Rollup / compilado denormalizado (quando dashboards leem agregado pré-calculado):**
- O contexto mantém um **compilado de janela móvel** (total/contagem/participação % por entidade num período)? Se sim, declarar: a semântica de cada métrica (o que conta como total; contagem = unidades vs ocorrências; base da %) — isso é **decisão de negócio** (vem do PRD; se ambíguo vs legado, reverse-engineer o cálculo legado e confirmar com o dev); a janela e o filtro de estado; **em qual tabela** os campos vivem (pode ser tabela de outro contexto — então o contexto que calcula é **escritor exclusivo daquelas colunas**, o dono só lê); e que o recompute é **best-effort** após a ingestão do raw (re-sync periódico recompila). `gofi-eng` faz recompute **set-based** (1 `UPDATE`+CTE com roll-off + guard div-zero), não reset+N. Padrão em `.claude/expertise/ddd-architecture/application-vs-domain-service.md` §"Recompute de agregado derivado".

**Padrões de projeto** (perguntar só quando o contexto indicar):
- CQRS, Saga, Strategy, Factory, idempotência, event sourcing

**Perfil de acesso ao banco — uma resposta por tabela do agregado:**
- Perfil de write: `append-only` / `hot UPDATE` / `hot DELETE+INSERT` / `cold`
- Combinações de filtro previstas (do `/schemas` ou listagem estática) + ordenações default
- Workers cross-cutting (purge, archive, replicação seletiva, etc.) que filtrem por coluna específica nesta tabela
- A spec **declara** o perfil em §3 (modelo de dados) ou §4 (arquitetura) — `gofi-eng` usa pra escolher índices na migration; `gofi-qa` audita aderência. Sem perfil declarado, a migration não tem como decidir índice corretamente.
- Estratégia completa de índices, fillfactor e autovacuum por perfil em `.claude/sdk/<lang>/knowledge/postgres-index-strategy.md` (PostgreSQL).

**Cenário transacional do agregado (aggregate methods + concorrência):**
- Se o agregado tem **mutação multi-tabela atômica** (entidade-raiz + N
  dependentes + snapshot de auditoria), a spec deve declarar em §3 ou §4
  que o repo expõe `CreateAggregate` / `UpdateAggregate` /
  `DeleteAggregate` (em vez de N saves separados orquestrados pelo
  service). Sinaliza pro `gofi-eng` aplicar o padrão repository-aggregate
  (ver `.claude/sdk/<lang>/knowledge/repository-aggregate-pattern.md`).
- **Isolation level**: assumir `ReadCommitted` por default — a spec
  **não precisa** declarar level específico. Spec só sinaliza isolation
  mais forte (`RepeatableRead` / `Serializable`) **quando há invariante
  cross-row** que o schema (UNIQUE/CHECK) não cobre — incluir uma RN
  numerada explicando a invariante. Ausência = `ReadCommitted` no código.
- **Consumer bulk previsto?** Se a spec lista consumer de carga em massa
  para o agregado (importação de planilha, sincronização batch,
  replicação), declarar explicitamente: "o repo expõe
  `CreateAggregatesBulk(ctx, []*Aggregate) error` além de `CreateAggregate`,
  e o caller é responsável por chunking". Sem essa declaração, `gofi-eng`
  **não cria** o bulk method (YAGNI). Detalhes do pattern em
  `.claude/sdk/<lang>/knowledge/repository-aggregate-pattern.md`.

**Segurança adicional:**
- Rate limiting por usuário/IP
- Auditoria de operações
- Encriptação em repouso

**Variáveis de ambiente:**
- O contexto usa integração externa fora do SDK (DB, cache, mensageria já cobertos)?
- **Apenas pergunte sobre variáveis fora do padrão** — `DATABASE_*`, `CACHE_*`, `MESSAGING_*`, `APP_*`, `OTEL_*`, `PORT`, `ALLOWED_ORIGINS`, `JWT_SECRET`, `JWT_ISSUER`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `OAUTH_GOOGLE_*` **já são SDK-padrão** (módulos Auth/OAuth/HTTP do `gofi/base/environment`) — não perguntar nem documentar como "fora do padrão"
- Variável genuinamente fora do padrão (ex: `STRIPE_API_KEY`, `SENDGRID_API_KEY`, IDPs além de Google): confirmar antes de incluir
- Padrão completo em `.claude/sdk/<lang>/knowledge/env-vars-standard.md`

**Auth (quando o contexto envolve login/sessão):**

> Os valores técnicos vêm prontos do SDK (`environment.Instance().Auth()` /
> `.OAuth()`): `JWT_SECRET`, TTLs (defaults 15m/168h), `Issuer` (fallback
> AppName), credenciais Google. **Não perguntar TTLs nem secret na elicitação.**
> A spec só decide políticas de domínio:

- Cookies HTTP-only (recomendado para web)?
- IDPs externos além de Google (Microsoft, OIDC genérico)? Google já é SDK-padrão — basta sinalizar que o contexto usa
- Multi-tenant (subdomínio, header, JWT claim, campo na entidade)?
- RBAC? Quais papéis e permissões?
- Política de revogação de refresh token (rotação? blacklist?)
- Recovery flow (reset password, change password)?

### Fase 5 — Confirmação do modelo

Antes de gerar a spec, apresente um resumo estruturado e peça confirmação:

```
### Entendi os seguintes pontos:

**Serviço:** {nome}
**Localização:** {backend/nome/}
**Module path:** {github.com/org/.../backend/nome}
**Banco:** {PostgreSQL | ...} | **Porta:** {8080}
**Prefixo:** {/api/v1} | **Serviço novo?** {sim/não}

**Contexto:** {nome em inglês} — tabela `{singular}`
**Linguagem ubíqua:** {termos principais}

**Entidade principal:** {nome}
| Campo | Tipo | Obrigatório | Validações |
|-------|------|-------------|------------|
| ...   | ...  | ...         | ...        |

**Value objects aninhados:** {lista ou "nenhum"}

**Endpoints:**
| Método | Path | Acesso |
|--------|------|--------|
| ...    | ...  | ...    |

**Regras de negócio:** {lista numerada}
**Segurança:** {cookies? refresh token? IDP? RBAC? multi-tenant?}

**Decisões de arquitetura:**
- Cache: {...} | Mensageria: {...} | Filtro: {estático|dinâmico|não}
- Padrões: {...} | Variáveis adicionais: {...} | Integrações: {...}

Está correto?
```

### Fase 6 — Geração da spec

Com o modelo confirmado, gere a spec em `specs/{contexto}/sdd-{contexto}.md`
seguindo o template em `.claude/templates/sdd-template.md`.
