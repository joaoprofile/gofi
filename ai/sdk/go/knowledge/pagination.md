---
name: pagination
description: Paginação com sqln — PageRequest 0-indexed, defaults, Page[T] JSON, sort seguro, contagem, lista simples e streaming
sdk: v0.8.2
keywords: [pagination, PageRequest, Page, PagedList, WithPage, NewPageRequest, sort, List, All, WithCountQuery]
---

# Paginação — `sqln` / `sqln/pagination`

API: `.claude/sdk/go/api/sqln-pagination.md` e `sqln.md`. Exemplo que percorre
todas as páginas: `examples/sqln/search/main.go`.

## Três formas de ler N linhas — escolha pela spec

| Forma | Retorno | Quando |
|---|---|---|
| `.List()` | `([]T, error)` | lista **simples**, pequena e limitada pelo domínio |
| `.WithPage(p).PagedList()` | `(*sqln.Page[T], error)` | lista **paginada** para o front |
| `.All()` | `iter.Seq2[T, error]` | volume grande processado linha a linha (export, job); sem cache nem página |

Nunca simule lista simples com `WithPage(NewPageRequest(0, 1000, …))` +
descarte do envelope: paga um `COUNT(*)` que ninguém lê e esconde um limite
mágico.

## PageRequest

- `sqln.NewPageRequest(page, limit, []sqln.Sort{...})` — `page`
  **0-indexed**: `page=0, limit=5` → `OFFSET 0 LIMIT 5`; `page=1` →
  `OFFSET 5`. A API expõe `page` 0-indexed; o repositório repassa sem
  converter.
- `limit=0` → `sqln.DefaultLimit` (15). Teto (`limit` máximo) é normalização
  do repositório (`read-endpoints.md`).
- `sqln.NewSort(campo, sqln.ASC|sqln.DESC)`; várias `Sort` viram
  `ORDER BY a DESC, b ASC`.
- **Campo de sort precisa ser referência de coluna** (`e.created_at`,
  `total`, `"createdAt"`): expressão ou texto injetado é **descartado** com
  warning. Nunca repasse o texto do cliente como campo — allowlist no
  repositório (ou `Sortable` do `FilterMapping` no filtro dinâmico).
- Filtro dinâmico: `sqln.NewPageRequestFilter(filters, mapping)` lê
  `params` do body e resolve o `sortField` pelo mapping (`dynamic-filter.md`).
- Com `criteria`, ORDER BY/LIMIT vêm do `PageRequest` — não use
  `OrderBy`/`Limit`/`Offset` junto de `WithPage`.

## `sqln.Page[T]` — envelope JSON

```json
{
  "totalPages": 2,
  "totalElements": 10,
  "size": 5,
  "number": 0,
  "numberOfElements": 5,
  "content": [ ... ]
}
```

`number` é a página pedida (0-indexed), `size` o limit aplicado. O handler
devolve a `Page` como veio — sem reembrulhar.

## Contagem

`PagedList` roda duas queries: `SELECT COUNT(*) FROM (<query>) tb` e a
página. Quando parte da query não muda o total mas custa por linha (ex.:
`LeftJoinLateral` só de projeção), informe a contagem com
`.WithCountQuery(sqlDeContagem, args...)` — usada como está, deve devolver uma
linha com um inteiro.

## Armadilhas

- `page=1` esperando a primeira página — documentar 0-indexed na spec.
- `criteria.Contains` é `ILIKE` no PostgreSQL (sem `.ToLower()`), mas **não**
  põe curinga: passe `"%"+termo+"%"`. (O filtro dinâmico com `LIKE` já
  envolve em `%…%`.)
- Paginação por `OFFSET` degrada em páginas muito profundas; para varredura
  total use `.All()`.
