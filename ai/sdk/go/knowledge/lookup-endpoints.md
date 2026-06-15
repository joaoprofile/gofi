# Lookup endpoints — dropdowns dos filtros dinâmicos

Padrão para popular **dropdowns / multi-selects do front** quando um campo do
`{Ctx}QueryMapping` é uma **lista de valores conhecidos** (enum interno,
lookup cross-context).

Aplicado quando o contexto usa filtro dinâmico (ver
[`dynamic-filter.md`](dynamic-filter.md)) e algum campo de `AllowedFields`
representa um conjunto fechado (ou semi-fechado) de valores.

> **Mudança v2 (2026):** o `FieldMapping` ganhou o campo `Content` e o
> `SearchType: "embedded"`. **Não existe mais endpoint dedicado `/status`** —
> a resposta de `getSchema` já carrega os valores inline para enums
> embedded, e aponta direto para o path do endpoint pra lookups
> cross-context. Implementações que ainda têm `GET /{ctx}/status` retornando
> "map de maps" são legado e devem ser removidas em refactor (o front
> migra para ler `allowedFields[i].content`).

---

## FilterType — tipo da busca

| FilterType | Significado | Operadores típicos | Exemplo |
|---|---|---|---|
| `text` | substring / `ILIKE` | `LIKE`, `=`, `IN` | título, SKU |
| `number` / `numeric` | comparação numérica | `=`, `<`, `>`, `BETWEEN` | preço, quantidade |
| `boolean` | flag | `=` | `is_archived` |
| `search-multiple` | lookup multi-valor (front renderiza multi-select) | `IN`, `NOT IN`, `=` | status (múltiplos), marketplace (múltiplos) |
| `search-single` | lookup single-valor (front renderiza select / radio) | `=`, `!=` | agente único habilitado |

`search-multiple` vs `search-single` é decisão de UX/produto, **não** de
domínio. O front renderiza um widget diferente, mas ambos disparam o
mesmo lookup (`SearchType`).

---

## SearchType — onde o front busca os valores

Dois modos, mutuamente exclusivos:

### 1. `SearchType: "embedded"` — valores inline na schema response

Usar quando o lookup é **estático** (enum fechado, sem consulta a DB) e
**cabe** na resposta do `getSchema`. O campo `Content` carrega o objeto
literal que o front renderiza direto.

```go
{
    Key:        "p.<status_col>",
    Label:      "STATUS",
    FilterType: "search-multiple",
    SearchType: "embedded",
    Content:    <enumPackage>.StatusMap,   // map[string]string ou struct/slice
},
```

Resultado em `POST /{ctx}/schemas`:

```json
{
  "key": "p.<status_col>",
  "label": "STATUS",
  "filterType": "search-multiple",
  "searchType": "embedded",
  "Content": { "ACTIVE": "ACTIVE", "ARCHIVED": "ARCHIVED" }
}
```

Front lê `field.content` direto — **zero round-trip extra**.

**Quando usar embedded:**
- Enum interno do contexto (status, tipo, modo)
- Cross-context se o enum também é estático (ex.: `AgentStatusMap` compartilhado)
- Cardinalidade pequena (até dezenas; centenas vira `api-path`)

### 2. `SearchType: "<api-path>"` — path do endpoint que retorna os valores

Usar quando o lookup é **dinâmico** (vem de DB, depende de tenant, paginável,
ou cardinalidade alta). O valor é o **path relativo da API** (sem `/`
inicial) que o front chama para buscar os valores.

```go
{
    Key:        "p.<fk_col>",
    Label:      "MARKETPLACE_ID",
    FilterType: "search-multiple",
    SearchType: "v1/account/marketplaces",   // path da API; front chama GET /v1/account/marketplaces
    // Content: omitido (nil)
},
```

**Quando usar api-path:**
- Lookup cross-context vivo no DB (marketplaces, accounts, sellers)
- Valores filtrados por tenant
- Cardinalidade alta — não cabe na schema response
- Lookup precisa de busca textual / paginação

O endpoint apontado é **propriedade do contexto-dono** — não do contexto
atual. Aqui só guardamos a referência.

---

## Content — formato do payload embedded

`Content` é `any` no SDK (`sqln.FieldMapping.Content any`). O formato
canônico é `map[string]string` (`map[CODE]LABEL`) para preservar
compatibilidade com clients que esperam o legado "map de maps":

```go
var StatusMap = map[string]string{
    StatusActive:   StatusActive,    // {CODE: CODE} quando o front faz i18n
    StatusArchived: StatusArchived,
}
```

Front que faz **i18n** lê só as chaves (códigos) e traduz no cliente.
Front que prefere label do server pode aceitar `{CODE: "Human Label"}` —
mas a regra geral do projeto é **enums em UPPER_SNAKE_CASE com front
traduzindo** (ver `feedback_enum_codes_upper_snake_i18n` na memória).

Tipos válidos para `Content`:
- `map[string]string` — formato canônico para enum simples
- `[]string` — quando ordem importa e front itera
- struct / slice de struct — quando o front precisa de metadata extra
  (ícone, cor, grupo). Manter o shape estável entre versões.

---

## Origem dos valores — pacote único `services/common/enums/`

O projeto usa **pacote único `enums`** com arquivos temáticos:

```
services/common/enums/
├── {resource1}.go    // {Resource1}Status*, {Resource1}Type*, ...
├── {resource2}.go    // {Resource2}Status*, ...
├── shared.go         // enums cross-context (sem prefixo de recurso)
└── ...
```

**Layout canônico por arquivo:**

```go
package enums

const (
    XxxStatusActive   = "ACTIVE"
    XxxStatusArchived = "ARCHIVED"
)

var XxxStatuses = []string{XxxStatusActive, XxxStatusArchived}

var XxxStatusMap = map[string]string{
    XxxStatusActive:   XxxStatusActive,
    XxxStatusArchived: XxxStatusArchived,
}

func IsValidXxxStatus(s string) bool {
    _, ok := XxxStatusMap[s]
    return ok
}
```

Prefixo do recurso na constante (`{Resource}Status*`) evita colisão em
pacote único. Quando vários contextos usam o **mesmo** enum, declarar
**uma vez** e referenciar a mesma constante (single source of truth).

> **Variação aceita:** projetos que ainda têm `services/common/{contexto}/`
> (um pacote por contexto, sem prefixo nas constantes) continuam válidos.
> A escolha é por consistência do repo, não dogma.

Por que **slice + map** juntos:

- Slice serve a validação (`oneof=ACTIVE ARCHIVED`), iteração ordenada e
  testes determinísticos
- Map serve ao `Content` embedded (resposta JSON da schema) e ao `IsValid`
  em O(1)

**Anti-padrão:** construir o map dinamicamente a partir do slice em cada
request — desperdício e perde a fonte única de verdade.

---

## Mapping — exemplo completo

```go
AllowedFields: []sqln.FieldMapping{
    // text livre — sem SearchType, sem Content
    {Key: "p.title", Label: "TITLE", FilterType: "text"},

    // enum interno embedded (múltiplos valores selecionáveis)
    {
        Key:        "p.<status_col>",
        Label:      "STATUS",
        FilterType: "search-multiple",
        SearchType: "embedded",
        Content:    enums.XxxStatusMap,
    },

    // enum embedded mas única seleção (UX decide)
    {
        Key:        "p.<agent_col>",
        Label:      "AGENT_STATUS",
        FilterType: "search-single",
        SearchType: "embedded",
        Content:    enums.AgentStatusMap,
    },

    // lookup cross-context dinâmico — front chama o path
    {
        Key:        "p.<fk_col>",
        Label:      "MARKETPLACE_ID",
        FilterType: "search-multiple",
        SearchType: "v1/account/marketplaces",
        // Content nil — front busca em runtime
    },
},
```

---

## Handler — schema endpoint só, sem `/status`

O `getSchema` já retorna o `Content` quando o struct é serializado — basta
**não filtrar** o campo. O JSON do `sqln.FieldMapping` carrega
`{key, label, filterType, searchType, Content}` automaticamente.

```go
func (h *XxxHandler) getSchema(w http.ResponseWriter, _ *http.Request) {
    qm := model.XxxQueryMapping()
    netx.Response(w, http.StatusOK, map[string]any{
        "allowedSortingFields": qm.AllowedSortingFields,
        "allowedFields":        qm.AllowedFields,   // inclui Content embedded
        "operators":            qm.Operators,
        "logicalOperators":     qm.LogicalOperators,
    })
}
```

Regras:

- **Sem handler `getStatus`** — código novo não cria; legado existente deve
  sair em refactor
- Endpoint `getSchema` **não consulta DB** (resposta vem 100% de constantes
  + map referenciado via `Content`)
- **Sem cache server-side** — payload é trivialmente pequeno e o
  `Cache-Control` do front já lida (TTL longo é seguro)

---

## Spec — onde declarar

Na spec SDD do contexto:

- **§4 (endpoints):** documentar **apenas** `POST /{prefixo}/{ctx}/schemas`
  + `POST /{prefixo}/{ctx}/query` — **não** declarar `/status`
- **§4.1.a (mapping):** para cada campo, declarar `FilterType` + (quando
  aplicável) `SearchType` + `Content` (referência à constante)

  | Key | Label | FilterType | SearchType | Content |
  |---|---|---|---|---|
  | `p.title` | TITLE | `text` | — | — |
  | `p.<status_col>` | STATUS | `search-multiple` | `embedded` | `enums.XxxStatusMap` |
  | `p.<fk_col>` | MARKETPLACE_ID | `search-multiple` | `v1/account/marketplaces` | — |

- **§3 (regras de negócio):** apontar de qual constante (`enums.*` ou
  `common/{ctx}/...`) cada enum vem — o `gofi-eng` consulta isso para
  preencher `Content`

---

## Anti-padrões

❌ Endpoint `GET /{ctx}/status` em código novo:
```go
netx.GET("/{ctx}/status").To(h.getStatus)   // padrão antigo, removido
```

❌ Hard-codar valores no handler em vez de referenciar a constante:
```go
Content: map[string]string{"FOO": "FOO", "BAR": "BAR"},  // duplicado de enums.*
```

❌ `SearchType: "embedded"` sem `Content` populado — front fica sem dados:
```go
{FilterType: "search-multiple", SearchType: "embedded"}  // Content faltando
```

❌ `Content` populado mas `SearchType != "embedded"` — confuso, front
ignora:
```go
{SearchType: "v1/account/marketplaces", Content: enums.SomeMap}
```

❌ `SearchType` com `/` inicial ou URL absoluta:
```go
SearchType: "/v1/account/marketplaces"     // sem barra inicial
SearchType: "https://api.example.com/..."  // path relativo da API, não URL
```

❌ Construir map a partir do slice em cada request:
```go
m := make(map[string]string, len(enums.XxxStatuses))
for _, s := range enums.XxxStatuses { m[s] = s }   // use o map já declarado
```

❌ Misturar `search-multiple` e `search-single` no mesmo campo sem critério:
escolha consciente baseada em UX (multi-select vs select único).

---

## Checklist (gofi-eng)

Antes de fechar a implementação:

- [ ] Cada campo enum no `{Ctx}QueryMapping` tem `FilterType: "search-multiple"`
      ou `"search-single"` (não `text`)
- [ ] Cada campo `search-*` tem `SearchType` não-vazio
- [ ] `SearchType: "embedded"` ⇒ `Content` referencia constante exportada
      (não literal inline)
- [ ] `SearchType: "v1/..."` ⇒ `Content` é `nil` / omitido
- [ ] `SearchType` de api-path **não** tem `/` inicial
- [ ] Constantes + slice + map declarados em `services/common/enums/{topico}.go`
      (ou `services/common/{ctx}/{ctx}.go` se o projeto usa pacote por contexto)
- [ ] `IsValidXxx()` deriva do map (não duplica switch case)
- [ ] **Nenhum** handler `getStatus` novo — se a spec antiga ainda menciona,
      apontar como divergência ao /gofi-qa
- [ ] Spec §4 e §4.1.a refletem `FilterType` + `SearchType` + `Content`
