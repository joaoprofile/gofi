---
name: dynamic-filter
description: Filtro dinâmico em Go — allowlist sqln.FilterMapping, BuildQuery, NewPageRequestFilter, tenant como argumento, envelope sqln.Filters
sdk: v0.8.2
keywords: [dynamic-filter, FilterMapping, BuildQuery, FindWithFilter, NewPageRequestFilter, ErrInvalidFilter, Filters, tenant]
---

# Filtro dinâmico — Go

Quando o cliente escolhe os filtros (em vez de query params fixos). API:
`.claude/sdk/go/api/sqln-filter.md` e `sqln.md` (`FilterMapping`,
`BuildQuery`, `NewPageRequestFilter`, `FindWithFilter`, `ErrInvalidFilter`).
Exemplo executável completo: `examples/sqln/filter-api`
(`product/repository.go` + `product/handler.go`).

## Endpoints

- `POST /{ctx}s/schemas` — devolve o `sqln.FilterMapping` do contexto (o que
  pode ser filtrado/ordenado e como o front monta a tela).
- `POST /{ctx}s/query` — recebe `sqln.Filters` no body e devolve
  `*sqln.Page[{Ctx}Query]`.

## A allowlist — `sqln.FilterMapping`

```go
// model/query_dto.go
var {Ctx}FilterMapping = sqln.FilterMapping{
    "name":    {Column: "e.name", Ops: sqln.Text, Sortable: true, Label: "NAME", FilterType: "text"},
    "status":  {Column: "e.status", Ops: sqln.Equality, Label: "STATUS",
        FilterType: "search-multiple", SearchType: "embedded", Content: enums.{Resource}StatusMap},
    "created": {Column: "e.created_at", Ops: sqln.Range, Sortable: true, Label: "CREATED_AT"},
}
```

- **Chave = nome de API** que o cliente envia; `Column` = expressão SQL
  (`json:"-"`, nunca vai ao cliente). Campo, operador ou sort fora do mapping
  → a requisição inteira é rejeitada. Não existe mais validação separada no
  handler: quem valida é `BuildQuery` / `NewPageRequestFilter`.
- `Ops`: `sqln.Text` (texto), `sqln.Equality` (enum, id, flag), `sqln.Range`
  (número, data). Vazio aceita todos — evite.
- `Sortable: true` só no que pode ordenar (e que tenha índice que sirva).
- `Label`, `FilterType`, `SearchType`, `Content` são metadados de UI,
  repassados intactos — regras em `lookup-endpoints.md`.
- **Nunca** coloque no mapping coluna que não é devolvida ao cliente (hash,
  segredo, flag interna): a allowlist existe para impedir sondagem do tipo
  `password_hash LIKE 'a%'`.
- `sqln.AllowColumns("e.a", "e.b")` mapeia coluna→ela mesma, todos os
  operadores e sort: só para migrar contrato legado que já envia colunas.

## Envelope do request — `sqln.Filters`

```json
{
  "params":  {"page": 0, "limit": 15, "sortField": "created", "sortDirection": "DESC"},
  "filters": [
    {"field": "status", "condition": "IN", "value": ["ACTIVE", "PAUSED"]},
    {"logicalOperator": "AND"},
    {"field": "name", "condition": "LIKE", "value": "abc"}
  ]
}
```

- Paginação só em `params`: `page` (0-indexed), `limit` (0 → 15), `sortField`
  (**nome de API** Sortable), `sortDirection` (`ASC`/`DESC`). Nunca `size`,
  `sortingFields[]` ou `page` na raiz.
- `filters` é lista plana; conectores são elementos próprios
  (`{"logicalOperator": "AND"|"OR"}`), nunca no início, no fim ou dois
  seguidos. Sem conector entre dois filtros = `AND`.
- `condition` é o operador literal: `=` `!=` `<` `<=` `>` `>=` `IN` `NOT IN`
  `LIKE` `NOT LIKE` `BETWEEN` `IS NULL` `IS NOT NULL` (constantes `sqln.Eq`,
  `sqln.In`, `sqln.Contains`, …). Alias (`"eq"`, `"contains"`) é rejeitado.
- `LIKE`/`NOT LIKE` = contém, **case-insensitive** (`ILIKE` no PostgreSQL); o
  SDK envolve o valor em `%…%` — o cliente manda só o termo.
- `BETWEEN` de datas: `"inicioRFC3339|fimRFC3339"`. Lista em `IN`/`NOT IN`.
  `value` nulo = `IS NULL`.

## Handler — parse, tenant do auth, delega

```go
func (h *{Ctx}Handler) getSchema(w http.ResponseWriter, _ *http.Request) {
    netx.Response(w, http.StatusOK, model.{Ctx}FilterMapping)
}

func (h *{Ctx}Handler) getDynamicQuery(w http.ResponseWriter, r *http.Request) {
    tenantID, ok := tenantFromCtx(r) // do token, nunca do body
    if !ok {
        netx.Error(w, http.StatusUnauthorized, errUnauthorized)
        return
    }
    var filters sqln.Filters
    if err := netx.ParseRequestBody(w, r, &filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }
    if len(filters.Filters) == 0 { // default só quando o cliente não filtrou
        filters.Add(sqln.NewFilter("status", sqln.Eq, enums.{Resource}StatusActive))
    }
    page, appErr := h.svc.GetByDynamicQuery(r.Context(), tenantID, &filters)
    if appErr.Exists() {
        netx.RespondError(w, r, appErr)
        return
    }
    netx.Response(w, http.StatusOK, page)
}
```

- Filtro default usa **nome de API** do mapping, com constante do enum
  (`lookup-endpoints.md` §"Origem dos valores").
- **`filters.Tenant` não é lido pelo SDK** e não tem tag `json` — o body
  consegue preenchê-lo. Nunca leia tenant de `Filters`; ele vai como argumento
  explícito do service/repository.

## Service — repassa e traduz o erro de filtro

```go
func (s *{ctx}Service) GetByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.{Ctx}Query], errs.AppError) {
    page, err := s.repo.FindByDynamicQuery(ctx, tenantID, f)
    if errors.Is(err, sqln.ErrInvalidFilter) {
        return nil, Err{Ctx}InvalidFilter.Wrap(err) // errs.RegisterValidation → 400
    }
    if err != nil {
        return nil, Err{Ctx}Query.Wrap(err)
    }
    return page, errs.AppError{}
}
```

## Repository — base com tenant em `$1`, BuildQuery, página

```go
const {ctx}DynamicQueryBase = `SELECT ` + {ctx}QuerySelectFields + `
FROM {tabela} e
WHERE e.tenant_id = $1`

func (r *{ctx}Repository) FindByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.{Ctx}Query], error) {
    if f.Params == nil {
        f.Params = &sqln.FilterParams{}
    }
    if f.Params.SortField == "" { // default qualificado pelo mapping
        f.Params.SortField, f.Params.SortDirection = "created", string(sqln.DESC)
    }
    q, err := sqln.BuildQuery({ctx}DynamicQueryBase, []any{tenantID}, f, model.{Ctx}FilterMapping, nil)
    if err != nil {
        return nil, err
    }
    page, err := sqln.NewPageRequestFilter(f, model.{Ctx}FilterMapping)
    if err != nil {
        return nil, err
    }
    return sqln.FindWithFilter[model.{Ctx}Query](ctx, q).WithPage(page).PagedList()
}
```

- A base termina **dentro do `WHERE`** — o SDK anexa `AND ( <filtros> )`,
  com os filtros do cliente num parêntese próprio. Tenant é **argumento
  ligado** (`$1`); os placeholders dos filtros continuam depois dos `args`.
  Um `OR` do cliente nunca escapa do predicado de tenancy. Sem tenancy
  (recurso genuinamente global): base termina em `WHERE TRUE` e `args` `nil`.
- `dialect` `nil` = dialeto da conexão ativa.
- **Sort default no repositório** (normalização é do repo): sem `sortField`,
  o SDK ordena por `id` sem qualificador, que fica ambíguo quando a projeção
  junta tabelas com `id`. Defina um default Sortable do mapping.
- Base query e `{ctx}QuerySelectFields` são **constantes de pacote**, nunca
  inline no método. Read model `{Ctx}Query` separado da entidade quando a
  projeção difere.
- Cache opcional: `.WithCache(sqln.NewCache[model.{Ctx}Query]("{ctx}:query:"+tenantID, ttl))`
  antes do `PagedList` — o SDK já separa as entradas por SQL + args + página
  (`cache-layer.md`).
- Export do mesmo filtro: `report-export.md`.

## Spec — o que declarar

- §0.1 Decisões: `Filtro dinâmico | sim — POST /{ctx}s/schemas + POST /{ctx}s/query`.
- §0.1 Contratos:
  ```go
  FindByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.{Ctx}Query], error)      // repository
  GetByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.{Ctx}Query], errs.AppError) // service
  ```
- §4: tabela do mapping (nome de API, coluna, `Ops`, `Sortable`, `Label`,
  `FilterType`, `SearchType`, `Content`), sort default e filtro default.

## Testes de handler

Body com envelope completo e operadores literais:
`{"params": {"page": 0, "limit": 15, "sortField": "name", "sortDirection": "ASC"},
"filters": [{"field": "name", "condition": "=", "value": "x"}]}`. Cubra o
400: campo fora do mapping, operador fora de `Ops`, `sortField` não Sortable.
