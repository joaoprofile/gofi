---
name: read-endpoints
description: Endpoints de leitura para o front — handler bind+validate, repo com criteria+cache+paginação, normalização no repository
type: feedback
sdk: v0.8.2
keywords: [read-endpoint, GET, BindQueryParamsToStruct, criteria, FindFromCriteria, PagedList, WithCache, pagination]
---

# Endpoints de leitura (GET) consumidos pelo front

Padrão para qualquer rota `GET` de listagem/detalhe servida ao front. Mantém o
handler fino, o service só com regra de negócio, e o "como consultar" (SQL,
paginação, cache, normalização de query) no repository. API:
`.claude/sdk/go/api/netx.md`, `sqln.md`, `sqln-criteria.md`.

## Handler — bind em struct + validate, nunca `strconv` cru

**Anti-padrão:** `limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))` (sem
validação, erro engolido).

**Padrão:** DTO com tags `form` (bind) + `validate` (shape):
```go
var req model.{Ctx}ListRequest
if err := netx.BindQueryParamsToStruct(r, w, &req); err != nil {
    netx.Error(w, http.StatusBadRequest, err)
    return
}
if err := req.Validate(); err != nil {
    netx.Error(w, http.StatusBadRequest, err)
    return
}
req.TenantID = tenantFromCtx(r) // do token, nunca da query
```

```go
type {Ctx}ListRequest struct {
    TenantID string `form:"-"`
    Sort     string `form:"sort"  validate:"omitempty,oneof=total count"`
    Limit    uint16 `form:"limit" validate:"omitempty,lte=200"`
    Page     uint16 `form:"page"`
}
func (r {Ctx}ListRequest) Validate() error { return v.ValidateStruct(r) } // v = validator.New()
```

- `netx.BindQueryParamsToStruct` casa pela tag `form` (fallback: nome em
  minúsculas) e **não roda** as tags `validate` — daí o `Validate()`.
- **Tenancy `form:"-"`**, setada do auth depois do bind — senão
  `?tenantid=outro` vaza entre tenants.
- `Page`/`Limit` como `uint16` (casam com `sqln.NewPageRequest`).

## Repository — criteria + cache + paginação; normalização mora aqui

```go
const {ctx}ListCacheTTL = 5 * time.Minute

func (r *{ctx}Repository) List{Ctx}(ctx context.Context, f model.{Ctx}ListFilter) (*sqln.Page[model.{Ctx}Row], error) {
    sortCol := "total" // allowlist de ordenação
    if f.Sort == "count" {
        sortCol = "count"
    }
    limit := min(f.Limit, maxLimit) // 0 vira DefaultLimit (15) no NewPageRequest

    q := criteria.From("{tabela}", "e").
        Select({ctx}RowSelectFields).
        Where(
            criteria.Eq("e.tenant_id", f.TenantID),
            criteria.DateOnOrAfter("e.day", f.From),
            criteria.DateOnOrBefore("e.day", f.To),
        ).
        GroupBy("e.item_id").
        Having(criteria.Gt("SUM(e.value)", 0))

    page := sqln.NewPageRequest(f.Page, limit, []sqln.Sort{sqln.NewSort(sortCol, sqln.DESC)})
    return sqln.FindFromCriteria[model.{Ctx}Row](ctx, q).
        WithCache(sqln.NewCache[model.{Ctx}Row]("{ctx}:list:"+f.TenantID, {ctx}ListCacheTTL)).
        WithPage(page).
        PagedList()
}
```

- **`criteria`** sempre que a query cabe: `Select/Join/LeftJoin/LeftJoinLateral/Where/GroupBy/Having`
  + predicados (`Eq`, `In`, `Contains`, `Between`, `DateOnOrAfter`, `IsTrue`,
  `Group`+`Or` para OR isolado). **Não** use `OrderBy`/`Limit` junto de
  `WithPage` — ordenação e página vêm do `PageRequest`.
- **Contagem da página:** o SDK embrulha a query em
  `SELECT COUNT(*) FROM (<q>) tb`, correto com `GROUP BY`. Se a projeção tem
  custo que não muda o total (ex.: `LeftJoinLateral` só de projeção), passe
  uma contagem própria com `.WithCountQuery(sql, args...)`.
- **Sort:** `sqln.Sort.Field` precisa ser referência de coluna — expressão é
  descartada com warning (proteção contra injeção). Ordene por alias da
  projeção.
- **Cache no repository** (`cache-layer.md`): o `nome` do `NewCache` é o
  escopo de invalidação; o SDK já separa as entradas por SQL + args + página.
- **Um resultado** (detalhe, agregado sem `GROUP BY`): `.UniqueResult()` →
  `(*T, error)`, `nil, nil` sem linha.
- **Mapeamento:** tags `db` do `model.{Ctx}Row` = nomes (ou aliases) das
  colunas; ver `value-objects.md`.
- **Field-list / SQL longo → const de pacote** (`{ctx}RowSelectFields`),
  passada como **um** argumento a `Select(...)`. Inline só quando curto.

## Service — só regra de negócio; repassa cru

O service resolve regra de negócio (janela de data + fuso, autorização,
defaults do domínio) e repassa `Sort`/`Page`/`Limit` crus — allowlist e teto
são do repository.

```go
func (s *{ctx}Service) List{Ctx}(ctx context.Context, req model.{Ctx}ListRequest) (*sqln.Page[model.{Ctx}Row], errs.AppError) {
    from, to, appErr := s.resolveWindow(req.From, req.To)
    if appErr.Exists() {
        return nil, appErr
    }
    page, err := s.repo.List{Ctx}(ctx, model.{Ctx}ListFilter{TenantID: req.TenantID, From: from, To: to,
        Sort: req.Sort, Page: req.Page, Limit: req.Limit})
    if err != nil {
        return nil, Err{Ctx}Query.Wrap(err)
    }
    return page, errs.AppError{}
}
```

## Quando criteria NÃO cabe

- **Funções de janela** (`ROW_NUMBER()`, `SUM() OVER (...)`), CTE: SQL cru
  como constante de pacote + `sqln.Find[T](ctx, query, args...)` — compõe
  `WithPage`/`WithCache` igual. Tenant é **argumento** (`$1`), nunca
  `fmt.Sprintf`.
- **Cálculo de população inteira** (ranking ABC/Pareto por participação
  acumulada) não pagina — endpoint próprio com lista completa (ou `.All()`),
  nunca embutido numa lista paginada.
- **Filtro por campos arbitrários escolhidos pelo front:** `dynamic-filter.md`
  e `lookup-endpoints.md`.

## Campos herdados de um owner/config via tabela de associação (membro herda do dono)

Quando o DTO de leitura expõe campos que vêm de uma **config compartilhada por um
grupo de entidades** — onde **só o dono (owner) possui a linha de config** e os
demais (membros) a **herdam** via uma tabela de associação `{group}` —, resolva a
config **através da associação**, nunca por FK direta.

**Anti-padrão (zera membro silenciosamente):**
```sql
LEFT JOIN {config} c ON c.entity_id = e.id   -- só casa o OWNER; membro → NULL → COALESCE→0/default
```
O membro não tem linha própria em `{config}`, então o JOIN direto devolve NULL e o
`COALESCE` entrega `0`/default — bug silencioso (membro "sem regra" quando na
verdade herda a do dono).

**Padrão correto (owner e membros resolvem a config do dono):**
```sql
LEFT JOIN {group}  g ON g.entity_id = e.id
LEFT JOIN {config} c ON c.id = g.{config_fk}   -- a associação aponta pra config do dono
```
- Sem linha em `{group}` (entidade fora de qualquer grupo) → `c` NULL → `0`/default
  (= "sem config"). A **tabela de associação é a fonte da verdade** de "tem config?".
- **Performance:** resolva por **chave única** da associação (`UNIQUE(entity_id)`) +
  **PK** da config — não por subselect correlacionado (`(SELECT ... WHERE entity_id = e.id)`),
  que roda por linha na listagem paginada.

**Flags de papel são mutuamente exclusivos — cuidado com `is_member` do owner.**
Se a tabela de associação grava o **owner também com `is_member=true`** (comum:
o payload de criação marca toda entrada como membro do grupo, inclusive o dono),
o mapeamento direto marcaria o dono como membro. Derive o papel "membro **não-dono**":
```sql
COALESCE(g.is_owner, FALSE)                        AS is_owner_flag,
COALESCE(g.is_member AND NOT g.is_owner, FALSE)    AS is_non_owner_member_flag
```
Dono → `true/false`; membro → `false/true`; sem grupo → `false/false`. Documente a
derivação numa RN da spec (o `AND NOT is_owner` é não-óbvio).

**Dois "status" diferentes — não conflar.** É comum o DTO carregar dois estados:
- **status do "agente"/toggle por-entidade** — coluna na **própria** tabela da entidade
  (`e.{x}_agent_status`), tipicamente uma **projeção sincronizada** do status da config
  (escrita em todos os membros do grupo na mesma tx pelo contexto dono);
- **status da "regra"/config** — vive **uma vez** na config do owner (`c.status`).
Cada um vem da sua origem real; nomeie o campo conforme o **bounded context do dado**
(o prefixo/sufixo reflete o contexto dono da config) — um nome genérico que colida
com outro conceito do mesmo bloco gera confusão e vira candidato a rename caro depois.

**Config inativa ainda surfaça os valores?** Pausar/desabilitar a config geralmente
**não** apaga a linha de associação (só a exclusão apaga) — então a config continua
resolvível. **Se** o range/valores herdados devem aparecer mesmo com config
`PAUSED/DISABLED` (quem comunica inativo é o status-toggle) **ou** zerar, é **decisão
de produto** que a **spec declara** — sem declaração explícita, não inventar filtro
de status no JOIN.

> Este é um padrão de **leitura cross-bounded-context**: o contexto que lê é
> **consumidor downstream** das tabelas do contexto dono da config. Registre o
> acoplamento no `memory/contexts/{ctx-dono}.md` (write-semantics load-bearing:
> "toda config cria a linha do owner", "owner gravado com `is_member=true`",
> "disable não apaga associação") — para o dono não mudar a escrita sem análise
> de impacto no leitor.
