---
name: api-endpoint-rules
description: Regras de endpoints em Go — filtro dinâmico com FilterMapping, lookups, config de motor de decisão, GET do front
sdk: v0.8.2
keywords: [endpoint, handler, FilterMapping, dynamic-filter, lookup, read-endpoint, validation, netx]
---

# Regras — endpoints e validação de entrada

Índice; detalhe no arquivo apontado. API: `.claude/sdk/go/api/netx.md`,
`sqln-filter.md`, `sqln.md`.

- **Filtro dinâmico = allowlist `sqln.FilterMapping`.** O cliente envia nomes
  de API (nunca colunas); cada nome aponta para sua coluna (`Column`, nunca
  serializada), operadores aceitos (`Ops`: `sqln.Text`, `sqln.Equality`,
  `sqln.Range`) e `Sortable`. `sqln.BuildQuery` rejeita qualquer campo,
  operador ou sort fora do mapping com erro que embrulha
  `sqln.ErrInvalidFilter` → **400**. Tenant entra como argumento da base query
  (`$1`), nunca como filtro nem literal formatado. Detalhe:
  `dynamic-filter.md`.
- **Lookups dos filtros (dropdowns) vêm no próprio mapping.** `FilterType`
  (`search-multiple`/`search-single`), `SearchType` (`"embedded"` ou path da
  API sem `/` inicial) e `Content` (constante do enum quando embedded) são
  metadados de UI de `sqln.FilterField`, serializados pelo endpoint que
  devolve o mapping. Sem endpoint `/status` à parte. Detalhe:
  `lookup-endpoints.md`.
- **Config que alimenta motor de decisão — Create exige payload completo.**
  Todo campo que o motor lê é `required` no Create (discriminador, limites,
  estado, parâmetros), mais coerência cross-field (`gtfield`/`gtefield`).
  Zero legítimo em número → `*T` ou `min=0`, não `required`. Update parcial só
  quando a spec declara; na dúvida, `PUT` completo. Detalhe: `validation.md`
  §"Config que alimenta motor de decisão".
- **GET do front — handler bind+validate; repo com criteria, cache e
  paginação.** Handler nunca faz `strconv.Atoi(q.Get(...))`: bind em DTO com
  `netx.BindQueryParamsToStruct` (tags `form`) + `DTO.Validate()`; campo de
  tenancy `form:"-"`, setado do auth depois do bind. Repository usa
  `criteria` (`GroupBy`/`Having` para agregação) + `WithCache` +
  `WithPage(sqln.NewPageRequest(...))` → `PagedList()`, ou `.UniqueResult()`
  para um resultado. Normalização (allowlist de sort, teto de limit) mora no
  repository; o service só tem regra de negócio e repassa crus. Detalhe:
  `read-endpoints.md`.
- **Export (XLSX/CSV) reusa o filtro dinâmico** do contexto e lê com
  streaming (`.All()`) — `report-export.md`.
- **Erros no handler:** erro de parse/bind → `netx.Error(w, http.StatusBadRequest, err)`;
  erro do service (`errs.AppError`) → `netx.RespondError(w, r, appErr)`, que
  mapeia o tipo registrado (`RegisterValidation` → 400, `RegisterNotFound` →
  404, `RegisterConflict` → 409, …). Padrão geral em `error-handling.md`.
