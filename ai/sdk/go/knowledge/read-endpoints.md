---
name: read-endpoints
description: Endpoints de leitura para o front — handler bind+validate, repo com criteria+cache+paginação, normalização no repository
type: feedback
---

# Endpoints de leitura (GET) consumidos pelo front

Padrão para qualquer rota `GET` de listagem/detalhe servida ao front. Mantém o
handler fino, o service só com regra de negócio, e o "como consultar" (SQL,
paginação, cache, normalização de query) no repository.

## Handler — bind em struct + validate, nunca `strconv` cru

**Anti-padrão:**
```go
q := r.URL.Query()
limit, _ := strconv.Atoi(q.Get("limit"))   // sem validação, erro engolido
page, _ := strconv.Atoi(q.Get("page"))
```

**Padrão:** DTO com tags `form` (bind) + `validate` (regras de shape), e o handler
faz bind + validate:
```go
var req model.XxxRequest
if err := netx.BindQueryParamsToStruct(r, w, &req); err != nil {
    netx.Error(w, http.StatusBadRequest, err)
    return
}
if err := req.Validate(); err != nil {        // só quando há tags validate
    netx.Error(w, http.StatusBadRequest, err)
    return
}
req.TenantID = authFromContext(r).CompanyID   // tenancy vem do auth, nunca da query
```

DTO:
```go
type XxxRequest struct {
    TenantID string `form:"-"`                                  // form:"-" → nunca bindado da query
    Sort     string `form:"sort"  validate:"omitempty,oneof=gmv units"`
    Limit    uint16 `form:"limit" validate:"omitempty,lte=200"`
    Page     uint16 `form:"page"`
}
func (r XxxRequest) Validate() error { return v.ValidateStruct(r) } // v = validator.New()
```

- `BindQueryParamsToStruct` (de `gofi/netx`) bind por tag `form` (fallback nome
  lowercased); suporta string/int/uint/slice. **Não roda os tags `validate`** —
  por isso o `Validate()` é chamado separado.
- **Campo de tenancy = `form:"-"`** e setado a partir do auth **depois** do bind —
  senão `?tenantid=outro` vazaria cross-tenant.
- Page/Limit como `uint16` (casa com `sqln.NewPageRequest`).

## Repository — criteria + cache + paginação; normalização mora aqui

A normalização de query (whitelist de sort, clamp de limit, offset) é
**responsabilidade do repository**, não do service. `sqln.NewPageRequest` já cuida
de offset e default de limit (page 0-indexed, `limit=0` → `DefaultLimit`).

```go
func (r *repo) GetXxx(ctx context.Context, f model.XxxFilter) (*sqln.Page[model.Row], error) {
    sortCol := "gmv"                    // whitelist de colunas ordenáveis (anti-injection)
    if f.Sort == "units" { sortCol = "units" }
    limit := f.Limit
    if limit == 0 { limit = defLimit }
    if limit > maxLimit { limit = maxLimit }

    q := criteria.From("xxx_table", "").
        Select("id", "COALESCE(MAX(name),'') AS name", "SUM(v) AS v").
        Where(criteria.Eq("tenant_id", f.TenantID)).
        Where(criteria.Gte("d", f.From)).Where(criteria.Lte("d", f.To)).
        GroupBy("id").Having(criteria.Gt("SUM(v)", 0))

    page := sqln.NewPageRequest(f.Page, limit, []sqln.Sort{sqln.NewSort(sortCol, sqln.DESC)})
    cache := sqln.NewCache[model.Row](fmt.Sprintf("xxx:%s:%s:%d:%d", f.TenantID, sortCol, f.Page, limit), cacheTTL)
    return sqln.FindFromCriteria[model.Row](ctx, q).WithCache(cache).WithPage(page).PagedList()
}
```

- **`criteria` builder** (não `fmt.Sprintf` + `QueryContext`) sempre que a query
  cabe: `Select/Where/Join/GroupBy/Having/OrderBy` + predicados
  `Eq/Gte/Lte/Gt/Between`. `GroupBy` + agregação é suportado; o `BuildCount`
  embrulha em subquery (`SELECT COUNT(*) FROM (<q>) t`), então a contagem da
  paginação fica correta mesmo com `GROUP BY`.
- **Cache no repository** (`sqln.NewCache[T]` + `.WithCache`), nunca no service
  (ver `cache-layer.md`). Chave inclui todos os params (tenant + filtros + page/limit).
- **Resultado paginado** = `*sqln.Page[T]` via `.WithPage(...).PagedList()`.
  Resultado único (agregação sem GROUP BY, ou detalhe) = `.Execute()` → `(*T, error)`.
- **Contrato posicional do `sqln`**: a struct `model.Row` tem tags `db` e a ordem
  dos campos = ordem das colunas no `Select` (scan posicional). Ver `value-objects.md`.
- **Field-list / SQL longo → const de pacote no repository** (nunca inline no
  método). Mesmo padrão de `configSelectFields`: uma `const xxxSelectFields = ...`
  multi-linha com vírgulas, passada como **um** argumento a `Select(xxxSelectFields)`
  (o builder não re-junta — a string já tem as vírgulas). Vale para qualquer
  string SQL grande (select, base de query raw, etc.): extrair melhora leitura,
  reuso e diff. Inline só quando é curto (1–2 colunas).

## Service — só regra de negócio; repassa cru

O service resolve **regra de negócio** (janela de data + fuso, autorização de
tenancy, defaults semânticos do domínio) e **repassa os params de query crus**
(`Sort`, `Page`, `Limit`) ao repo — a normalização mecânica é do repo.

```go
func (s *svc) GetXxx(ctx, req) (*sqln.Page[model.Row], errs.AppError) {
    from, to, appErr := s.resolveWindow(req.From, req.To) // regra de negócio (tz, máx, futuro)
    if appErr.Exists() { return nil, appErr }
    page, err := s.repo.GetXxx(ctx, model.XxxFilter{TenantID: req.TenantID, From: from, To: to,
        Sort: req.Sort, Page: req.Page, Limit: req.Limit}) // cru — sem clamp/whitelist aqui
    if err != nil { return nil, ErrXxxQuery.Wrap(err) }
    return page, errs.AppError{}
}
```

## Quando criteria NÃO cabe

- **Funções de janela** (`ROW_NUMBER()`, `SUM() OVER (...)`) não existem no
  `criteria` builder → usar base raw + `sqln.FindWithFilter[T](ctx, sqln.NewQueryBuild(base, filters))`
  (padrão do filtro dinâmico `/schemas` + `/query`), que também compõe
  `WithPage`/`WithCache`. Tenancy entra como literal via `fmt.Sprintf(base, filters.Tenant)`.
- **Cálculos de população inteira** (ranking ABC/Pareto via `cum_share`) **não
  paginam** — o `cum_share` de uma página é sem sentido. São endpoint próprio
  (lista completa) ou cômputo separado, nunca embutidos numa lista paginada.

## Filtro dinâmico (quando o front filtra por campos arbitrários)

Quando o front precisa filtrar/ordenar por campos variados, é o padrão de filtro
dinâmico (`POST /{ctx}/schemas` + `POST /{ctx}/query` com `sqln.Filters`) — ver
`dynamic-filter.md` e `lookup-endpoints.md`.

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
