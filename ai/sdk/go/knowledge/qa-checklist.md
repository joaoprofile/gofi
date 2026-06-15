# Checklist de Auditoria — Go

Aplicado por `gofi-qa` em todo contexto Go implementado.

## 1. Conformidade com a Spec
- [ ] Todos os campos da entidade estão implementados
- [ ] Todas as operações listadas na spec existem
- [ ] Todas as regras de negócio (RN-*) estão implementadas
- [ ] HTTP status codes correspondem ao mapeado na spec
- [ ] Filtros de listagem se comportam como especificado

## 2. Padrões gofi/sqln
- [ ] Repository usa statements preparados (`sqln.NewStatement().Prepare`) para mutations — `*sql.Stmt` em campo do struct (`stmInsertX`, `stmUpdateX`, `stmDeleteX`), preparado **uma única vez** em `New{Contexto}Repository(ctx)`. `sqln.NewStatement().Execute(ctx, sql, args...)` inline em mutation (prepara + executa + descarta a cada chamada) é **MAJOR** — quebra cache de prepare e cria round-trip extra. Exceção: SQL dinâmico montado em runtime (filtro dinâmico) não pode ser preparado.
- [ ] **Helpers de persistência são métodos do receiver** — nenhuma função no arquivo do repo tem assinatura `func xxx(ctx context.Context, ...) error` executando SQL. Todas são `func (r *{contexto}Repository) ...`. Helper como `func insertConfig(ctx, e) error` solto no pacote (sem receiver) é **MAJOR** — perde acesso aos stmts preparados no struct e borra a fronteira de encapsulamento. Funções **puras** sem `ctx`/I/O (ex.: `configArgs(e *Config) []any`) podem ficar como funções de pacote.
- [ ] Dentro de `r.tx.Execute(...)`, helpers fazem rebind via `ctx.Value(connection.SqlTxContextKey).(*sql.Tx).Stmt(r.stmXxx)` antes de `ExecContext` — chamar `r.stmXxx.ExecContext` direto pega outra conexão do pool e não participa da transação (**BLOCKER** — quebra atomicidade silenciosamente).
- [ ] `Close()` fecha **todos** os `*sql.Stmt` do struct em sequência, retornando o primeiro erro.
- [ ] Entidade usa tags `db:"col_name"` — **nunca `gofi:""`** (tag errada, não é mapeada)
- [ ] `sqln.Filters.Tenant` é `int32` — nunca assumir `int64`
- [ ] Paginação usa `sqln.NewPageRequest(page, limit, sorts)` com `page` 0-indexed
- [ ] Criteria usa `criteria.From(table, alias).Select(...).Where(...)`
- [ ] `FindByID` retorna `(*Entity, error)` — `nil, nil` quando não encontrado
- [ ] **Consultas de presença** (`ExistsByXxx`) retornam **`(*T, error)`** direto do `FindFromCriteria[T].Execute()` — service consome via `if exists != nil { ... }` (ver `repository-primitive-return.md`)
- [ ] Nunca usa `database/sql` diretamente
- [ ] **Value objects aninhados** (ver `value-objects.md`):
  - Campo externo tem tag `db:""` (marcador de presença)
  - Sub-campos têm tags `db:""` e a ordem corresponde à ordem das colunas
  - VO multi-coluna → struct simples (mapper expande recursivamente)
  - VO em coluna única (JSON) → implementa `sql.Scanner`/`driver.Valuer`

## 3. Padrões gofi/base/errs
- [ ] Erros do service em `errors.go` registrados com `errs.Register*`
- [ ] Service retorna `errs.AppError` — nunca `error` puro
- [ ] Erros de validação usam `ErrXxxValidation.WithDetails(err)`
- [ ] Not-found em Update: detectado via `FindByID` retornando nil **antes** de chamar `repo.Update` — não via `ErrNoRowsAffected`
- [ ] Erros de operação usam `ErrXxxAction.Wrap(err)`

## 4. Padrões gofi/base/validator
- [ ] DTOs têm `Validate()` chamando `v.ValidateStruct(r)`
- [ ] Validator é instância de pacote (`var v = validator.New()`)
- [ ] Tags `validate:"..."` cobrem todas as RNs de validação

## 5. Padrões gofi/netx
- [ ] Body JSON via `netx.ParseRequestBody(w, r, &req)`
- [ ] Query params via `netx.BindQueryParamsToStruct(r, w, &f)` (tag `form:"field"`)
- [ ] Path param via `netx.GetPathParam("id", r)`
- [ ] Response via `netx.Response(w, status, data)`
- [ ] Erros 400/404/409/500 via `netx.RespondError(w, appErr)`
- [ ] Erros 401/403 via `netx.Error(w, http.StatusXxx, err)` — **nunca** `netx.RespondError`
- [ ] Handler **não** contém lógica de negócio

## 6. Separação de Camadas
- [ ] Handler não acessa repository diretamente
- [ ] Service não conhece `http.ResponseWriter` / `http.Request`
- [ ] Repository não conhece DTOs

## 7. Testabilidade
- [ ] Service recebe interface de repository
- [ ] Handler recebe interface de service
- [ ] Service test cobre: sucesso, validação inválida, not-found, erro de repo
- [ ] Handler test cobre: sucesso, decode error, service error mapeado
- [ ] Mocks handcraft (fn fields no service test, campos de retorno no handler test) — sem frameworks

## 8. Segurança
- [ ] Sem SQL concatenado — sempre parâmetros posicionais (`$1`, `$2`, ...)
- [ ] Sem dados sensíveis em log (senha, token, CPF completo)
- [ ] Sem erros internos vazando em respostas HTTP
- [ ] IDs de rota validados antes de uso (UUID format quando aplicável)

## 9. Logging
- [ ] Usa `logging.*` do `gofi/obs/logging` — nunca `fmt.Println`/`log.*`
- [ ] Erros fatais na inicialização: `logging.Fatal`
- [ ] Campos estruturados: `slog.Any("key", val)` — não interpolação

## 10. Wiring (main.go + estrutura)
- [ ] `main.go` em `pathCmd` (`./src/{projectName}/main.go`) — não na raiz de `pathService`
- [ ] `go.mod` em `pathService` (`./src/go.mod`) — não em `pathCmd`
- [ ] `domain/` em `pathService` — não dentro de `pathCmd`
- [ ] `.migrations/` em `pathService` (`./src/.migrations/`)
- [ ] `go.work` usa `pathService` (`use ./src`)
- [ ] Replace paths no `go.mod` apontam para `../gofi`
- [ ] Constantes `APP_NAME` e `APP_PORT` definidas (não literais inline)
- [ ] `var AllowedOrigins` declarada
- [ ] Wiring após Build: `repository.New(ctx)` → `service.New(repo)` → `handler.New(svc)`
- [ ] Registro via `api.HttpServer().AddHandlers(handler)` — não via `.Handlers()` no builder chain
- [ ] `api.ListenAndServe()` é a última instrução
- [ ] **Cron com horário fixo** (`cronjob.Fixed` Hour/Minute): `LocationName` IANA **explícito** (fuso de negócio, não tz do container) — `Hour:0` sem location roda em UTC silenciosamente. **MINOR** se faltar location explícito num job "noturno"
- [ ] Binário que usa `time.LoadLocation`/`cronjob.Fixed` com nome IANA importa `_ "time/tzdata"` — **MAJOR**: sem isso `ScheduleJob` **panica no boot** em imagem slim/scratch/distroless sem tzdata do SO (ver `worker-bootstrap.md` §"Cron com horário fixo")

## 11. Qualidade Geral
- [ ] Sem dead code (funções não chamadas, vars não usadas)
- [ ] Sem magic strings repetidas — extrair para constantes
- [ ] Sem TODO/FIXME sem rastreamento
- [ ] `go vet` e `golangci-lint` passam sem warnings

## 12. Variáveis de Ambiente
- [ ] `os.Getenv(...)` usa nomes do padrão gofi (`DATABASE_*`, `CACHE_*`, `MESSAGING_*`, `APP_*`, `OTEL_*`, `SERVICE_DEBUG_*`, `CLOUD_*`)
- [ ] Variáveis fora do padrão (`REDIS_ADDR`, `DB_HOST`, etc.) são MAJOR — desvio do SDK
- [ ] Exceções legítimas (IDP externo, terceiros) documentadas explicitamente na spec
- [ ] Ver `env-vars-standard.md` para a tabela completa

## 13. Índices e perfil de acesso ao banco (PostgreSQL)

Ver `postgres-index-strategy.md` para o catálogo completo (perfis, padrões
por tipo de filtro, fillfactor, autovacuum, particionamento).

- [ ] Cada tabela do contexto tem perfil declarado na spec (`cold` / `hot UPDATE` / `hot DELETE+INSERT` / `append-only`). Ausente: **MAJOR**
- [ ] Multi-tenant: todo índice tem leading column = tenant (composite ou partial). Single-column em não-tenant: **MAJOR**
- [ ] `LIKE '%x%'` / `ILIKE` / regex em `text`: índice GIN com `gin_trgm_ops` (extensão `pg_trgm`). Btree single-column: **MAJOR**
- [ ] Hot UPDATE: índices em colunas voláteis minimizados (cada um quebra HOT update)
- [ ] Hot UPDATE com colunas indexadas estáveis: `fillfactor=70-80` aplicado
- [ ] Hot UPDATE / Hot DELETE+INSERT: autovacuum tunado (`vacuum_scale_factor=0.01` e similares)
- [ ] Worker cross-cutting (purge, archive, replicação) declarado no projeto: índice da coluna em **toda tabela** onde ela existe
- [ ] Append-only de alto volume: tabela particionada (`PARTITION BY RANGE (created_at)`); índices declarados no parent
- [ ] Boolean indexado sem partial: **MINOR**
- [ ] Drop+recreate de índice em migration de produção usa `CONCURRENTLY`

## 14. Filtro Dinâmico (apenas quando o contexto usa)

Ver `dynamic-filter.md` para o checklist específico (model `query_dto.go`,
handler com `queryMapping.Validate(filters)`, service que apenas repassa
`*sqln.Filters`, repository com `FindWithFilter` + `NewQueryBuild` +
`NewPageRequestFilter`, query base com predicate de tenancy ou `WHERE 1=1`
declarada como constante).

Checks de segurança específicos (auditar **sempre** que o contexto tem
tenancy + filtro dinâmico):

- [ ] **Tenant na base query, nunca em `filters.Filters`** — repository tem
  `WHERE p.<tenant_col> = %d` na constante base + `fmt.Sprintf(...,
  filters.Tenant)` no método. Handler **só** seta `filters.Tenant`. Se o
  handler faz `filters.Filters = append([]*sqln.Filter{NewFilter("p.<tenant_col>", ...)}, ...)`:
  **BLOCKER** — vazamento cross-tenant quando cliente envia `OR` no body
  (precedência `AND > OR` quebra o isolamento; o parêntese externo do SDK
  envolve o conjunto, não o tenant individual)
- [ ] `filters.Tenant` é setado pelo handler antes de chamar o service
- [ ] `<tenant_col>` (a coluna canônica de tenancy do projeto, ex.: `p.tenant_id`)
  **não** aparece em `AllowedFields` do `QueryMapping` — cliente não pode
  filtrar por tenant via body

## 15. Lookup endpoints — shape v2 do `FieldMapping` (apenas com filtro dinâmico)

Ver `lookup-endpoints.md` para o catálogo completo. O `FieldMapping` ganhou
`SearchType` + `Content`; **o endpoint dedicado `GET /{ctx}/status` foi
descontinuado** — front lê `allowedFields[i].content` direto da resposta
de `getSchema`.

- [ ] **`FilterType`** de cada campo enum é `search-multiple` ou
      `search-single`. Campo enum com `FilterType: "text"` é **MAJOR**
      (front não consegue renderizar dropdown)
- [ ] **`SearchType` não-vazio** em todo campo `search-*` — vazio é **MAJOR**
      (front não sabe de onde tirar os valores)
- [ ] **`SearchType: "embedded"` ⇒ `Content` populado** com constante
      exportada (ex.: `enums.XxxStatusMap`). `Content` nil ou literal map
      inline é **MAJOR**
- [ ] **`SearchType: "v1/..."` ⇒ `Content` nil/omitido** — ter `Content`
      junto com api-path é **MINOR** (confuso; front ignora)
- [ ] **`SearchType` de api-path sem `/` inicial** — `"/v1/..."` ou URL
      absoluta (`"https://..."`) é **MAJOR** (front concatena base
      incorretamente)
- [ ] **Nenhuma rota `getStatus`** no handler — código novo não cria.
      Presente em código novo: **MAJOR** (padrão descontinuado). Presente
      em código legado: **SUGGESTION** (refactor para remover quando o
      front migrar)
- [ ] **Constantes em `services/common/enums/{topico}.go`** (pacote único
      com prefixo nas constantes — `ProductStatusMap`, `AgentStatusMap`)
      ou `services/common/{contexto}/` se o repo já usa pacote por
      contexto. **Slice + map declarados juntos** (slice para
      `oneof`/iteração, map para `Content` + `IsValid`)
- [ ] **`IsValidXxx()` deriva do map** — switch case duplicado é **MINOR**
      (perde fonte única)
- [ ] **Reuso cross-context**: o mesmo enum embedded usado em N
      `QueryMapping`s referencia **a mesma constante** — redeclaração
      paralela é **MAJOR** (drift garantido na próxima evolução)
- [ ] **`getSchema` serializa `FieldMapping` inteiro** (incluindo
      `Content`) — filtrar campos no handler é **MAJOR** (front fica
      sem dados de embedded)

---

## Severidade

| Nível | Quando |
|-------|--------|
| **BLOCKER** | Impede funcionamento correto: SQL injection, panic em produção, retorno errado |
| **MAJOR** | Viola padrão gofi ou introduz bug latente: stmt não preparado, service retornando `error` puro |
| **MINOR** | Desvio de convenção sem impacto funcional: import desordenado, nome fora de padrão |
| **SUGGESTION** | Melhoria opcional: extrair constante, refinar mensagem |
