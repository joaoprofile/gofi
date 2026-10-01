---
name: lookup-endpoints
description: Valores de dropdown dos filtros dinâmicos — FilterType, SearchType embedded vs path, Content em sqln.FilterField, enums compartilhados
sdk: v0.8.2
keywords: [lookup, FilterMapping, FilterField, FilterType, SearchType, Content, embedded, enums, dropdown]
---

# Lookup endpoints — dropdowns dos filtros dinâmicos

Quando um campo do `{Ctx}FilterMapping` (ver `dynamic-filter.md`) é um
conjunto fechado ou semi-fechado de valores (enum, lookup de outro contexto).
Os metadados de UI moram no próprio `sqln.FilterField` (`Label`, `FilterType`,
`SearchType`, `Content`), que o SDK serializa sem tocar — API em
`.claude/sdk/go/api/sqln-filter.md` (`filter.Field`).

> Não existe endpoint `/status` à parte: o `POST /{ctx}s/schemas` devolve o
> mapping com `content` inline para enums embedded e com o path para lookups
> dinâmicos. `getStatus` em código novo é divergência.

## Resposta do schema

`netx.Response(w, http.StatusOK, model.{Ctx}FilterMapping)` serializa um
objeto indexado pelo **nome de API** (a coluna nunca sai):

```json
{
  "status": {
    "ops": ["=", "!=", "IN", "NOT IN", "IS NULL", "IS NOT NULL"],
    "label": "STATUS",
    "filterType": "search-multiple",
    "searchType": "embedded",
    "content": {"ACTIVE": "ACTIVE", "ARCHIVED": "ARCHIVED"}
  },
  "name": {"ops": ["=", "LIKE", "..."], "sortable": true, "label": "NAME", "filterType": "text"}
}
```

Campos vazios são omitidos (`omitempty`). O front lê `field.content` direto —
zero round-trip.

## FilterType — widget

| FilterType | Significado | `Ops` típico |
|---|---|---|
| `text` | substring | `sqln.Text` |
| `number` | comparação numérica | `sqln.Range` |
| `boolean` | flag | `sqln.Equality` |
| `search-multiple` | lookup multi-valor (multi-select) | `sqln.Equality` (`IN`/`NOT IN`) |
| `search-single` | lookup de um valor (select) | `sqln.Equality` |

`search-multiple` vs `search-single` é decisão de UX, não de domínio.

## SearchType — de onde vêm os valores

1. **`"embedded"` + `Content`** — enum estático, cardinalidade pequena (até
   dezenas): `Content` referencia a constante exportada.
2. **Path da API** (`"v1/{recurso}"`, sem `/` inicial, nunca URL absoluta) +
   `Content` nil — lookup vivo no banco, filtrado por tenant, paginável ou de
   alta cardinalidade. O endpoint é do contexto dono; aqui só a referência.

```go
"status": {Column: "e.status", Ops: sqln.Equality, Label: "STATUS",
    FilterType: "search-multiple", SearchType: "embedded", Content: enums.{Resource}StatusMap},
"owner":  {Column: "e.owner_id", Ops: sqln.Equality, Label: "OWNER",
    FilterType: "search-single", SearchType: "v1/{recurso}"},
```

Formato de `Content`: `map[string]string` (`{CODE: CODE}` quando o front faz
i18n) como padrão; `[]string` se a ordem importa; slice de struct quando o
front precisa de metadado extra (shape estável entre versões).

## Origem dos valores — `{pathService}/common/enums/`

Pacote único `enums`, um arquivo por tema, prefixo do recurso nas constantes:

```go
package enums

const (
    {Resource}StatusActive   = "ACTIVE"
    {Resource}StatusArchived = "ARCHIVED"
)

var {Resource}Statuses = []string{{Resource}StatusActive, {Resource}StatusArchived}

var {Resource}StatusMap = map[string]string{
    {Resource}StatusActive:   {Resource}StatusActive,
    {Resource}StatusArchived: {Resource}StatusArchived,
}

func IsValid{Resource}Status(s string) bool {
    _, ok := {Resource}StatusMap[s]
    return ok
}
```

- Slice serve a validação (`oneof`), iteração ordenada e testes; map serve ao
  `Content` e ao `IsValid` em O(1).
- Mesmo enum em N contextos = **uma** declaração, N referências.
- Variação aceita: pacote por contexto (`{pathService}/common/{contexto}/`)
  quando o repositório já usa esse layout — consistência do repo decide.
- Enum interno que nunca cruza fronteira pode ficar no `model/` do contexto.

## Spec

§4 documenta só `POST /{ctx}s/schemas` + `POST /{ctx}s/query`; a tabela do
mapping traz `FilterType`, `SearchType` e `Content` (nome da constante) por
campo. §3 aponta de qual constante cada enum vem.

## Anti-padrões

- Endpoint `GET /{ctx}/status` novo.
- `Content` literal inline em vez da constante do pacote `enums`.
- `SearchType: "embedded"` sem `Content`; `Content` com `SearchType` de path.
- `SearchType` com `/` inicial ou URL absoluta.
- Montar o map a partir do slice a cada request.
- Filtrar/remontar o mapping no handler — serialize a variável inteira.
- Consulta ao banco no `getSchema` (a resposta é 100% constante).

## Checklist

- [ ] Todo campo enum tem `FilterType` `search-multiple`/`search-single` e `SearchType` preenchido
- [ ] `embedded` ⇒ `Content` = constante exportada; path ⇒ `Content` nil, sem `/` inicial
- [ ] `Ops` do campo enum é `sqln.Equality`
- [ ] Constantes + slice + map + `IsValid` no pacote `enums`
- [ ] Nenhum handler `getStatus`
