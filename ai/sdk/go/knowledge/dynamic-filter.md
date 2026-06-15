# Filtro Dinâmico — Go

Aplicado quando o contexto precisa de filtros arbitrários montados pelo cliente
(em vez de query params fixos).

## Endpoints
- `POST /{ctx}s/schemas` — retorna `QueryMapping` (campos filtráveis, ordenáveis, operadores, lógicos)
- `POST /{ctx}s/query` — recebe `sqln.Filters` no body, valida contra o mapping, retorna `sqln.Page[{Ctx}Query]`

## Envelope JSON do request — `sqln.Filters`

Forma canônica do body de `POST /{ctx}s/query` (definida em
`gofi/sqln/filter/dynamic_filter.go` — `Filters` + `FilterParams` + `Filter`):

```json
{
  "params": {
    "page": 0,
    "limit": 15,
    "sortField": "<column-or-alias>",
    "sortDirection": "ASC"
  },
  "filters": [
    { "field": "<table>.<column>", "condition": "=", "value": "<scalar>" },
    { "logicalOperator": "AND" },
    { "field": "<table>.<column>", "condition": "IN", "value": [1, 2, 3] }
  ]
}
```

Regras invioláveis do envelope:

- **Paginação fica em `params`** — não no topo. Campos: `page` (uint16, default `0`),
  `limit` (uint16, default `15`), `sortField` (string), `sortDirection` (`"ASC"`/`"DESC"`,
  default `"ASC"`). **Nunca** `size`, **nunca** `sortingFields[]`, **nunca** `page`/`limit`
  no nível raiz.
- **Filtros são uma lista plana** com separadores lógicos como elementos próprios
  (`{ "logicalOperator": "AND" }`) — **não** estrutura aninhada.
- **`tenant` nunca vem do body** — handler injeta a partir do JWT.
- Operadores no campo `condition` são **strings SQL literais** (ver §"Convenção de
  operadores" abaixo).

Anti-padrões comuns (rejeitar em PR):

```json
{ "page": 0, "size": 15, "sortingFields": [...] }   // sem params, nomes errados
{ "params": { "size": 15 } }                          // size em vez de limit
{ "filters": [{ "operator": "eq", ... }] }            // alias em vez de SQL literal
```

## Convenção de operadores
**Strings SQL literais** — nunca aliases:
- `sqln.Eq = "="`
- `sqln.Contains = "LIKE"`
- `sqln.And = "AND"` / `sqln.Or = "OR"`

O cliente envia esses valores **exatos** no campo `condition` do filtro.
Testes que usam `"operator":"eq"` ou `"contains"` estão **errados**.

## Model — `query_dto.go`
- Arquivo **separado** de `dto.go` — não misturar
- `{Ctx}QueryMapping()` retorna `*sqln.QueryMapping` com `AllowedFields`, `AllowedSortingFields`, `Operators`, `LogicalOperators`
- `{Ctx}Query` struct usa tags `db:""` (read model — separado da entidade de escrita)
- `{Ctx}QueryResponse` é alias para `*sqln.Page[{Ctx}Query]`

### `FieldMapping` — shape canônico

```go
type FieldMapping struct {
    Key        string `json:"key"`
    Label      string `json:"label"`
    FilterType string `json:"filterType"`
    SearchType string `json:"searchType"`
    Content    any    `json:"Content"`
}
```

- **`FilterType`**: `text` | `number` | `boolean` | `search-multiple` | `search-single`
- **`SearchType`** (só para `search-multiple`/`search-single`):
  - `"embedded"` — valores inline via `Content` (enum estático); front consome direto sem round-trip
  - `"v1/<path>"` — path relativo da API (sem `/` inicial) que retorna os valores dinamicamente
- **`Content`** (só quando `SearchType == "embedded"`): a constante referenciada (`map[string]string` canônico, ou shape estável)

Campos `text` / `number` / `boolean` deixam `SearchType` e `Content` zero.
Detalhes, decisão `search-multiple` vs `search-single`, anti-padrões e
checklist em [`lookup-endpoints.md`](lookup-endpoints.md).

> **Não existe mais endpoint dedicado `/status`** — o front lê
> `allowedFields[i].content` direto da resposta de `getSchema`. Handler
> `getStatus` em código novo é divergência.

## Handler
- `getSchema` retorna o mapping como JSON
- `getDynamicQuery` chama `queryMapping.Validate(filters)` **antes** de chamar o service — validação é responsabilidade do handler, nunca do service
- `filters.Tenant` é injetado pelo handler a partir do JWT — **nunca** vem do body, **nunca** aparece em `AllowedFields`, **nunca** é prependido em `filters.Filters` (ver §"Tenant não vai em `filters.Filters`" abaixo)
- Filtro default (`filters.Add(...)`) aplica no **handler** quando `len(filters.Filters) == 0` — depois do `Validate`, antes de chamar o service
- Erro de `Validate(filters)` retorna `netx.Error(w, http.StatusBadRequest, err)` — não `RespondError`

### Esqueleto canônico do `getDynamicQuery`

```go
func (h *XxxHandler) getDynamicQuery(w http.ResponseWriter, r *http.Request) {
    tenantID, ok := tenantFromCtx(r)             // extrai do JWT (pode ser companyID, accountID, etc.)
    if !ok {
        netx.Error(w, http.StatusUnauthorized, errors.New("unauthorized"))
        return
    }
    filters := &sqln.Filters{}
    if err := netx.ParseRequestBody(w, r, filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }
    if err := model.XxxQueryMapping().Validate(filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }

    filters.Tenant = tenantID                     // (1) tenancy via campo Tenant; repository materializa na base query
    if len(filters.Filters) == 0 {                // (2) default só quando cliente não filtrou nada
        filters.Add(sqln.NewFilter("p.<status_col>", sqln.Eq, sharedConst.StatusActive))
    }

    page, appErr := h.svc.GetByDynamicQuery(r.Context(), filters)
    if appErr.Exists() { netx.RespondError(w, appErr); return }
    netx.Response(w, http.StatusOK, page)
}
```

### Tenant não vai em `filters.Filters` — vai na base query (security)

**Regra inviolável:** o predicate de tenancy (`p.tenant_col = X`) é injetado
como **literal int** na string da base query do repository, **nunca** como
elemento de `filters.Filters`. Setar `filters.Tenant` no handler é
**obrigatório** (campo dedicado, lido pelo repo); prependar `NewFilter("p.tenant_col", ...)`
em `filters.Filters` é **anti-padrão de segurança**.

**Por quê.** O `sqln.NewQueryBuild` envolve **toda** a lista de filtros do
cliente em **um único parêntese externo** (`fmt.Sprintf("%s AND ( %s )", base, clause)`).
Se o tenant for prependido como Filter junto dos filtros do cliente, e o cliente
mandar um `OR` no body (legítimo pelo envelope), o SQL fica:

```sql
WHERE 1=1 AND ( p.tenant_col = $1 AND p.name LIKE $2 OR p.sku = $3 )
```

Pela precedência SQL (`AND` > `OR`), isso é avaliado como
`( (tenant AND name) OR sku )` — linhas com `sku = 'BAR'` de **outro tenant**
são retornadas. **Vazamento cross-tenant.**

**Como fazer certo.** Tenant entra na base query do repository como literal:

```go
// repository
const xxxDynamicQueryBase = `SELECT ` + xxxQuerySelectFields + `
FROM xxx p
WHERE p.tenant_col = %d`              // %d para int / %s para UUID já validado

func (r *xxxRepository) FindByDynamicQuery(ctx context.Context, f *sqln.Filters) (...) {
    base := fmt.Sprintf(xxxDynamicQueryBase, f.Tenant)   // f.Tenant vem do JWT, tipo numérico — sem injection
    return sqln.FindWithFilter[model.XxxQuery](ctx,
        sqln.NewQueryBuild(base, f),
    ).WithPage(sqln.NewPageRequestFilter(f)).PagedList()
}
```

Resultado: `WHERE p.tenant_col = 123 AND ( <filtros do cliente, OR seguro entre eles> )`
— o parêntese do SDK isola o `OR` do cliente sem afetar o predicate de tenancy.

**Tenant UUID (string).** Se `Tenant` for UUID, **não** use `%s` cru — o valor
veio do JWT mas a categoria de risco é a mesma; valide com `uuid.Parse` antes
e formate com aspas: `fmt.Sprintf("WHERE p.tenant_col = '%s'", parsed.String())`.
Tipo numérico (`int32`/`int64`) dispensa validação extra.

**Anti-padrões a rejeitar em PR:**
- `filters.Filters = append([]*sqln.Filter{NewFilter("p.tenant_col", Eq, tenantID)}, filters.Filters...)`
- Restringir `LogicalOperators` a só `AND` no mapping para "consertar" o problema — reduz expressividade do filtro dinâmico e o buraco volta na primeira mudança de mapping
- Confiar no parêntese externo do SDK para isolar tenant — ele isola **o conjunto**, não o tenant individualmente

### Filtro default com enum compartilhado

Quando o default precisa de uma constante de domínio (ex.: status "ativo"
enquanto outros estados só aparecem se o cliente pedir explicitamente), a
constante **não vai inline** no handler nem isolada no `model/` do contexto.
Vai num pacote compartilhado em `{pathService}/common/{contexto}/{contexto}.go`
(canônico do SDK — pacote per-contexto sob `common/` quando o enum é
referenciado cross-context):

```go
package {contexto}

const (
    StatusActive    = "ACTIVE"   // usado no filtro default da listagem
    StatusArchived  = "ARCHIVED"
    StatusPaused    = "PAUSED"
)

var Statuses = []string{StatusActive, StatusArchived, StatusPaused}

func IsValid(s string) bool {
    switch s {
    case StatusActive, StatusArchived, StatusPaused:
        return true
    }
    return false
}
```

Quando criar esse pacote (gatilhos):
- O **handler** precisa do valor para o filtro default (ex.: `StatusActive`)
  **e** o domínio também precisa (entidade, repository, service test).
  Inline em só um lugar = duplicação garantida quando o segundo consumidor surgir.
- O enum tem >2 valores e qualquer um deles é referenciado em mais de um arquivo Go.
- Outro contexto futuro vai consumir o mesmo enum (ex.: agent que valida transição
  de status antes de publicar evento).

Quando **não** criar (mantém local no `model/`):
- Enum interno do contexto que nunca cruza fronteira (`BatchOperationStatus*` típico).
- Apenas o handler precisa, em um único `if`. Não vale o pacote ainda.

**Caminho físico canônico:** `{pathService}/common/{contexto}/{contexto}.go`
(arquivo único; sem subdiretórios). Em Go: `import "{module}/common/{contexto}"`.

## Service
- `GetByDynamicQuery(ctx, *sqln.Filters)` na interface
- Implementação **passa `*sqln.Filters` direto ao repository** — sem validar, sem transformar
- Usa `ErrXxxQuery` existente (mesmo do `GetByFilter`) — não cria novo erro
- Import `"github.com/joaoprofile/gofi/sqln"` necessário

## Cache em listagem paginada (`PagedList` + `.WithCache`)

- **`PagedList()` honra `.WithCache`** — `ExecutePagedQuery` faz get na entrada
  e set no sucesso, cacheando a `*sqln.Page[T]` inteira. Encadeie inline (mesmo
  padrão (a) single-query de `cache-layer.md`):
  ```go
  return sqln.FindWithFilter[model.{Ctx}Query](ctx, sqln.NewQueryBuild(base, f)).
      WithCache(sqln.NewCache[model.{Ctx}Query](key, ttl)).
      WithPage(sqln.NewPageRequestFilter(f)).
      PagedList()
  ```
  O tipo do `NewCache` é o **row type** (`model.{Ctx}Query`), **não**
  `sqln.Page[...]` — o SDK hidrata a `Page` internamente (`cache.Get(ctx, &page)`).
- **A chave (`name` do `NewCache`) deve codificar tudo que muda o resultado** —
  tenant + filtros + page + sort —, porque o cache do SDK chaveia só pelo
  `name`. Monte com hash determinístico:
  `fmt.Sprintf("{ctx}:query:%s:%x", tenant, sha256.Sum256(json.Marshal(f)))`
  (`json.Marshal(f)` já inclui `f.Params` → page/limit/sort + `f.Filters`).
  Sem isso, combos de filtro/página diferentes colidem. TTL curto (filtros
  arbitrários = baixa taxa de hit; staleness aceitável se o dado é eventual).
  Invalidação por TTL — sem `Del` explícito (consistência eventual).
- **`NewPageRequestFilter` faz default de `sortField` vazio para `"id"` cru
  (não qualificado).** Se a base query tem `JOIN` e a tabela juntada também
  tem coluna `id`, `ORDER BY id` é **ambíguo → erro SQL**. Quando há JOIN,
  o handler **deve** setar um `sortField` qualificado default (`"p.id"`) +
  direção, **depois** do `Validate` (e incluir `p.id` em `AllowedSortingFields`).

## Repository
- `FindByDynamicQuery` usa `sqln.FindWithFilter[{Ctx}Query]` — **nunca** `FindFromCriteria`
- Query base com `sqln.NewQueryBuild(query, f)` (PostgreSQL) ou `NewQueryBuildWithDialect` para outros bancos
- Paginação com `sqln.NewPageRequestFilter(f)` — extrai de `f.Params`, **nunca** `NewPageRequest(page, limit, sorts)`
- Query base **deve terminar com um predicate** (`WHERE p.tenant_col = %d` quando há tenancy, ou `WHERE 1=1` quando não há) — `NewQueryBuild` anexa `AND (...)`, nunca `WHERE`. Ver §"Tenant não vai em `filters.Filters`" no Handler para a regra de segurança que define qual usar
- **Nenhum** `*sql.Stmt` no construtor para a query dinâmica — construída em runtime
- Query base declarada como **constante de pacote** no topo do arquivo:
  ```go
  // Com tenancy (padrão):
  const personDynamicQueryBase = `SELECT ` + personQuerySelectFields + ` FROM person p WHERE p.tenant_col = %d`
  // E no método: baseQuery := fmt.Sprintf(personDynamicQueryBase, filters.Tenant)

  // Sem tenancy (raro — só quando o recurso é genuinamente global):
  const personDynamicQuery = `SELECT ` + personQuerySelectFields + ` FROM person p WHERE 1=1`
  ```
  Nunca inline no método.
- Constante `{ctx}QuerySelectFields` separada de `{ctx}SelectFields` quando o read model difere da entidade

## Spec — campos obrigatórios

`§0.1 Decisões de Arquitetura`:
```
| Filtro dinâmico | sim — POST /{ctx}s/schemas + POST /{ctx}s/query |
```

`§0.1 Contratos de Camada`:
```go
// Repository
FindByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.{Ctx}QueryResponse, error)

// Service
GetByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.{Ctx}QueryResponse, errs.AppError)
```

`§4` deve ter seções dedicadas para `/schemas` e `/query` documentando:
- Campos filtráveis (Label, Key SQL, FilterType)
- Campos ordenáveis
- Filtro default

`§8 Estrutura de Arquivos`:
```
│   └── query_dto.go      # {Ctx}QueryMapping(), {Ctx}QueryResponse, {Ctx}Query
│   └── {ctx}_repository.go   # ...
│       #   {ctx}DynamicQuery = constante de pacote — nunca inline
```

## Testes de handler

`validQueryBody` deve usar o envelope completo (`params` + `filters`) com operadores
**SQL literais**:

```json
{
  "params": { "page": 0, "limit": 15, "sortField": "<col>", "sortDirection": "ASC" },
  "filters": [{ "field": "<table>.<col>", "condition": "=", "value": "<v>" }]
}
```

Não use `"operator":"eq"` (seria silenciosamente ignorado pelo predicate builder).
Não envie `page`/`limit` no nível raiz — o handler **só** lê de `params`.
