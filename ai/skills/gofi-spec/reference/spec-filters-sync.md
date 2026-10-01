# Filtro dinâmico, lookup endpoints e sync inbound na spec

### Filtro dinâmico na spec

Se o contexto usa filtro dinâmico, a spec deve incluir contratos e seções
adicionais — ver `.claude/sdk/<lang>/knowledge/dynamic-filter.md` para a
checklist exata (linhas em §0.1, contratos de Repository/Service, seções
§4 dos endpoints `/schemas` e `/query`, anotações em §8).

### Lookup endpoints (dropdowns dos filtros) na spec

O `sqln.FilterMapping` (chave = nome de API) carrega o lookup inline
(`Content`) ou aponta para o path do endpoint (`SearchType: "v1/..."`).
**Não existe mais endpoint dedicado `/status`** — `POST /{ctx}s/schemas`
devolve o mapping como objeto indexado pelo nome de API e o front consome
`schema[campo].content` direto da resposta.

- **§0.1 e §4** — declarar **apenas** `POST /{prefixo}/{ctx}/schemas` +
  `POST /{prefixo}/{ctx}/query`. **Não** adicionar `/status` (rota
  removida; código novo não cria, legado existente vira refactor).
- **§4.1.a (mapping)** — para cada campo, declarar `FilterType` + (quando
  enum/lookup) `SearchType` + `Content`, além de `Column` (expressão SQL,
  nunca serializada), `Ops` e `Sortable`. Tabela mínima:

  | Campo (API) | Column | Ops | Sortable | Label | FilterType | SearchType | Content |
  |---|---|---|---|---|---|---|---|
  | `title` | `p.title` | `Text` | sim | TITLE | `text` | — | — |
  | `status` | `p.<status_col>` | `Equality` | — | STATUS | `search-multiple` | `embedded` | `enums.XxxStatusMap` |
  | `agentStatus` | `p.<agent_col>` | `Equality` | — | AGENT_STATUS | `search-single` | `embedded` | `enums.AgentStatusMap` |
  | `categoryId` | `p.<fk_col>` | `Equality` | — | CATEGORY_ID | `search-multiple` | `v1/category/list` | — |

- **§3 (regras de negócio)** — apontar de qual constante (canônico:
  `services/common/enums/{topico}.go`; aceito: `services/common/{contexto}/`)
  cada enum embedded vem (mesma RN ou RNs separadas). `SearchType` de
  api-path **não** precisa de RN — só referencia o contexto-dono.

Detalhes, decisão `multiple` vs `single`, anti-padrões e checklist em
`.claude/sdk/<lang>/knowledge/lookup-endpoints.md`.

### Sync inbound multi-adapter — padrões reutilizáveis

Quando o contexto sincroniza dados **inbound** de N sistemas externos (cada um com API/webhook/report próprios), aplicar os padrões abaixo.

- **Bridge com capability per-adapter**: spec declara em §0.1 a interface única `{Ctx}Bridge` com **todos os métodos** (`FetchX/FetchY/...`). Cada adapter implementa **todos**, retornando `Err{Ctx}BridgeNotSupported` (registrado em `services/domain/{ctx}/bridge/errors.go`) nos métodos que o sistema externo não suporta. **Anti-padrão**: branchar por dimensão na application (`if dim == X ...`) — o saber fica no adapter, application chama plana.

- **`FetchX` retorna slice quando 1→N na borda**: quando 1 entidade externa expande em N locais (ex.: 1 entidade com variações vira N entidades achatadas no nosso domínio), o método retorna `[]Snapshot`. Spec declara em §0.1 quando o fan-out é por design.

- **Account-level event (`entity_kind=account`) — discovery por catálogo sem webhook**: para sistemas sem webhook de novas entidades, scheduler emite **1 evento por conta**. Consumer chama `FetchByAccount(token, accountID)` → catálogo → roteia cada item. Spec declara em §4 (Mensageria) qual processor enumera as contas — canônico: `services/domain/{ctx}/scheduler/repository.FindAccountsByDimension`. Application despacha por `EntityKind` (product vs account) **dentro** do handler do type principal.

- **Per-type apply split na application**: quando o orquestrador despacha por `event.Type` para handlers distintos, separar fisicamente em `apply_{type}.go` (mesma struct/receiver/package). Spec declara em §8: `application/{ctx}_application.go` (dispatch) + `application/apply_{type1}.go`, `apply_{type2}.go`, etc. Localidade > parcimônia; anti-padrão é 1 arquivo gigante com switch + N handlers inline.

- **Materialização-no-write vs read-join para enriquecimento de cache**: quando o agregado de leitura precisa de campos enriquecidos de uma tabela-cache (ex.: nome/reputação de entidade externa em ranking), a spec declara a decisão em §3 ou §4: **(a) materializar no write** (batch read da cache + grava colunas no agregado; misses preenchidos por enricher background) OU **(b) read-join** (agregado só armazena FK; leitura faz JOIN). Default para hot reads = (a). Ambos exigem tabela-cache + worker enricher.

- **Scheduler proativo com staleness filter**: scheduler **só emite** pra entidades cujo `max(notification_updated_at, scheduler_updated_at)` > TTL. Não re-processa o que webhook já atualizou. Spec declara em §4 (Mensageria) o TTL por (dim, event_type) via env e qual coluna lê.

- **Helpers reutilizáveis entre adapters**: quando ≥2 adapters do mesmo contexto compartilham lógica (truncar identificador, parse loose, escolher elemento de coleção por regra de domínio), promover ao `services/common/helpers/` (gen, não-domínio) ou ao `services/domain/{ctx}/model/` (ligado ao domínio). Spec declara em §8 quando uma função do modelo é compartilhada. Anti-padrão: helper privado replicado em N adapters.

- **Factory de bridge deve cachear instâncias**: spec declara em §0.1 que `factory.Get(key)` retorna **bridge cacheada** (1 HTTP client por key). Sem cache, split de consumer escala N×M clients independentes e estoura rate limit real do sistema externo. Padrão em `.claude/sdk/go/knowledge/bridge-factory-adapter-pattern.md`.

- **Observabilidade via `gofi/obs`**: spec declara em §0.1 que o contexto exporta métricas via package `services/common/observability/{ctx}/` (OpenTelemetry). Decorator pattern pra instrumentar bridges (zero acoplamento nos adapters), classifier centralizado pra mapear `errs.AppError → label fechado`, cardinality controlada (zero label free-form). Padrão completo em `.claude/sdk/go/knowledge/observability-otel.md`.

- **Consumer split por type quando há starvation cross-workload**: quando 1 consumer processa N types com perfis distintos (latência crítica vs tolerante), e métricas mostram que o type pesado satura workers e afeta os outros, splitar em N consumer groups (mesmo binário ou binários separados). Spec declara em ADR a estratégia faseada (single → split por type → binários separados → split de tópico) com **métricas-gatilho objetivas** pra cada promoção. Anti-padrão: splitar antes de medir.

