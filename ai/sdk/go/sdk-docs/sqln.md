# gofi/sqln — SQL Layer

## Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| `DATABASE_DRIVER` | `postgres`, `mysql`, `sqlserver`, `oracle` | `postgres` |
| `DATABASE_MIGRATION` | `true` para rodar migrations na inicialização | `false` |
| `DATABASE_HOST` | Host do banco | `localhost` |
| `DATABASE_USER` | Usuário | — |
| `DATABASE_PASSWORD` | Senha | — |
| `DATABASE_NAME` | Nome do banco | — |
| `DATABASE_PORT` | Porta | `5432` |
| `DATABASE_SSL_MODE` | `disable`, `require`, `verify-full` | `disable` |
| `DATABASE_MAX_OPEN_CONNS` | Máx conexões abertas | `25` |
| `DATABASE_MAX_IDLE_CONNS` | Máx conexões ociosas | `5` |
| `DATABASE_MAX_LIFETIME` | Lifetime máximo em segundos | `300` |

## Conexão

```go
import "github.com/joaoprofile/gofi/sqln"

// Inicializa conexão via env vars DATABASE_* (ver tabela acima)
sqln.Init(ctx)

// Migrations automáticas
sqln.Migrate(ctx, "migrations/")
```

## Statements Preparados

```go
stmt, err := sqln.NewStatement().Prepare(ctx, "INSERT INTO persons (name) VALUES ($1)")
if err != nil {
    logging.Fatal("prepare failed", slog.Any("error", err))
}

// Executar
_, err = stmt.ExecContext(ctx, "João")

// Fechar ao finalizar
stmt.Close()
```

Statements são criados no construtor do repository e reutilizados. Nunca prepare inline por request.

## Criteria Builder

```go
import "github.com/joaoprofile/gofi/sqln/criteria"

q := criteria.From("persons", "p").
    Select("p.id, p.name, p.email, p.cpf, p.age, p.created_at").
    Where(criteria.Eq("p.id", id))

// Executar → *T ou nil, nil quando não encontrado
person, err := sqln.FindFromCriteria[model.Person](ctx, q).Execute()
```

### Predicados disponíveis

```go
criteria.Eq("field", value)            // field = $N
criteria.Contains("field", "%val%")    // field ILIKE $N (case-insensitive)
criteria.In("field", []any{...})       // field IN ($N, ...)
criteria.And(pred1, pred2)
criteria.Or(pred1, pred2)
```

## Paginação

```go
import "github.com/joaoprofile/gofi/sqln"

page := sqln.NewPageRequest(
    f.Page,   // uint16, 0-indexed: page=0 → OFFSET 0, page=1 → OFFSET limit
    f.Limit,  // uint16, 0 → usa DefaultLimit (15)
    []sqln.Sort{
        sqln.NewSort("p.created_at", sqln.DESC),
    },
)

result, err := sqln.FindFromCriteria[model.Person](ctx, q).WithPage(page).PagedList()
// result é *sqln.Page[model.Person]
```

### sqln.Page[T]

```go
type Page[T any] struct {
    Data        []T   `json:"data"`
    Total       int64 `json:"total"`
    TotalPages  int   `json:"totalPages"`
    CurrentPage int   `json:"currentPage"`
    Limit       int   `json:"limit"`
}
```

### Constantes

```go
sqln.DefaultPage  = 0
sqln.DefaultLimit = 15
sqln.ASC          // SortDirection
sqln.DESC         // SortDirection
```

## Scan com gofi tags

```go
type Person struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CPF       string    `db:"cpf"`
    Age       int       `db:"age"`
    CreatedAt time.Time `db:"created_at"`
}
```

`sqln.FindFromCriteria` usa a tag `db:"col_name"` para mapear colunas SQL para campos do struct.

## Value Objects Aninhados

Structs aninhadas com tag `db` são expandidas recursivamente pelo mapper. Use quando um grupo de atributos faz sentido como um value object no domínio (ex: `Pricing`, `Address`, `Money`) e o banco guarda os valores em colunas simples.

```go
type Pricing struct {
    Price float64 `json:"price" db:"price"`
}

type Product struct {
    ID    int64   `json:"id"      db:"id"`
    Name  string  `json:"name"    db:"name"`
    Price Pricing `json:"pricing" db:"price"`
}
```

Query: `SELECT id, name, price FROM product` → a coluna `price` escaneia em `Product.Price.Price`.

### Regras

- A tag `db` no campo externo é **marcador de presença** — sem ela o campo é ignorado pelo mapper
- A **ordem dos sub-campos internos** define o mapeamento posicional com as colunas da query
- Suporta **múltiplos níveis** de aninhamento (ex: `A.B.C.valor` — o mapper desce até achar um tipo escaneável)
- `time.Time` e tipos que implementam `sql.Scanner` **não** são recursados — tratados como primitivos
- Slices com tag `db` usam `pq.Array` automaticamente — não recursam

## Transações

```go
sqln.Transaction(ctx, func(ctx context.Context) error {
    // ctx contém a *sql.Tx — repassar para todos os repos
    return repo.Save(ctx, person)
})
```

## CriteriaFrom vs From

```go
// Através do pacote sqln (sem importar criteria)
q := sqln.CriteriaFrom("persons", "p").Select(fields)

// Diretamente do sub-pacote (necessário para predicados)
import "github.com/joaoprofile/gofi/sqln/criteria"
q := criteria.From("persons", "p").Select(fields).Where(criteria.Eq("p.id", id))
```

## Cache (sqln.NewCache)

Cache é responsabilidade **exclusiva da camada de repository** — nunca do service.

```go
import "github.com/joaoprofile/gofi/sqln"

const cacheTTL = 10 * time.Minute

// Em FindByFilter no repository — WithCache antes de WithPage
cacheKey := fmt.Sprintf("product:%s:list:%s:%s:%d:%d", f.TenantID, f.SKU, f.Name, f.Page, f.Limit)
cache := sqln.NewCache[model.Product](cacheKey, cacheTTL)

return sqln.FindFromCriteria[model.Product](ctx, q).
    WithCache(cache).
    WithPage(page).PagedList()
```

### Invalidação de cache

Invalidação também fica no repository. Expor `InvalidateListCache` na interface:

```go
type ProductRepository interface {
    // ...
    InvalidateListCache(ctx context.Context, tenantID string)
}

func (r *productRepository) InvalidateListCache(ctx context.Context, tenantID string) {
    pattern := fmt.Sprintf("*product:%s:list:*", tenantID)
    keys, err := sqln.InstanceRedis().Keys(ctx, pattern).Result()
    if err != nil || len(keys) == 0 {
        return
    }
    sqln.InstanceRedis().Del(ctx, keys...)
}
```

O service chama `repo.InvalidateListCache(ctx, tenantID)` após mutations (Save, Update). Nunca acessa `sqln.InstanceRedis()` diretamente no service.

## Filtro Dinâmico (QueryMapping)

Permite que o frontend/client envie filtros arbitrários com validação server-side contra um mapping declarado.

### Tipos envolvidos

```go
import "github.com/joaoprofile/gofi/sqln"

// sqln.QueryMapping — declaração dos campos e operadores permitidos
type QueryMapping struct {
    AllowedSortingFields map[string]string   // campo lógico → "sortedBy"
    AllowedFields        []FieldMapping       // campos filtráveis
    Operators            map[string]string   // sqln.Eq, sqln.Contains, ...
    LogicalOperators     map[string]string   // sqln.And, sqln.Or
}

// sqln.FieldMapping — um campo filtrável
type FieldMapping struct {
    Key        string // alias SQL: "p.name"
    Label      string // identificador público: "NAME"
    FilterType string // "text", "number", "date", ...
}

// sqln.Filters — body da request de query dinâmica
type Filters struct {
    Filters []Filter // lista de filtros enviados pelo cliente
    Tenant  string   // tenant_id, injetado pelo middleware (não vem do body)
    // campos de paginação também podem existir
}

// sqln.Filter — um filtro individual
// criado via sqln.NewFilter(field, operator, value)
```

### Constantes de operadores

```go
sqln.Eq       // "="
sqln.Contains // "LIKE"  (ILIKE no PostgreSQL)
sqln.And      // "AND"
sqln.Or       // "OR"
```

> **Atenção:** os operadores são strings SQL literais (`"="`, `"LIKE"`, `"AND"`), não aliases amigáveis (`"eq"`, `"contains"`). O cliente envia exatamente esses valores no campo `condition` do filtro.

### Fluxo completo

**1. model/query_dto.go — mapeamento e tipo de resposta:**
```go
package model

import (
    "time"
    "github.com/joaoprofile/gofi/sqln"
)

func ProductQueryMapping() *sqln.QueryMapping {
    return &sqln.QueryMapping{
        AllowedSortingFields: map[string]string{
            "Name":  "sortedBy",
            "Price": "sortedBy",
        },
        AllowedFields: []sqln.FieldMapping{
            {Key: "p.name", Label: "NAME", FilterType: "text"},
            {Key: "p.sku",  Label: "SKU",  FilterType: "text"},
        },
        Operators: map[string]string{
            sqln.Eq:       sqln.Eq,
            sqln.Contains: sqln.Contains,
        },
        LogicalOperators: map[string]string{
            sqln.And: sqln.And,
            sqln.Or:  sqln.Or,
        },
    }
}

type ProductQueryResponse = *sqln.Page[ProductQuery]

type ProductQuery struct {
    ID        int64     `json:"id"        db:"id"`
    TenantID  string    `json:"tenantId"  db:"tenant_id"`
    SKU       string    `json:"sku"       db:"sku"`
    Name      string    `json:"name"      db:"name"`
    Price     float64   `json:"price"     db:"price"`
    Stock     int64     `json:"stock"     db:"stock"`
    Active    bool      `json:"active"    db:"active"`
    CreatedAt time.Time `json:"createdAt" db:"created_at"`
    UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}
```

**2. handler — rotas e métodos:**
```go
// Rotas adicionadas em Handlers()
netx.POST("/products/schemas").To(h.getSchema),
netx.POST("/products/query").To(h.getDynamicQuery),

// getSchema — expõe o mapping para o frontend
func (h *ProductHandler) getSchema(w http.ResponseWriter, r *http.Request) {
    qm := model.ProductQueryMapping()
    schema := map[string]interface{}{
        "allowedSortingFields": qm.AllowedSortingFields,
        "allowedFields":        qm.AllowedFields,
        "operators":            qm.Operators,
        "logicalOperators":     qm.LogicalOperators,
    }
    netx.Response(w, http.StatusOK, schema)
}

// getDynamicQuery — executa query com filtros do cliente
func (h *ProductHandler) getDynamicQuery(w http.ResponseWriter, r *http.Request) {
    filters := &sqln.Filters{}
    if err := netx.ParseRequestBody(w, r, filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }
    if err := model.ProductQueryMapping().Validate(filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }
    if len(filters.Filters) == 0 {
        filters.Add(sqln.NewFilter("p.active", sqln.Eq, true))
    }
    // TODO: filters.Tenant = claims.TenantID (quando IAM configurado)
    products, appErr := h.svc.GetByDynamicQuery(r.Context(), filters)
    if appErr.Exists() {
        netx.RespondError(w, appErr)
        return
    }
    netx.Response(w, http.StatusOK, products)
}
```

**3. repository — FindByDynamicQuery:**
```go
func (r *productRepository) FindByDynamicQuery(ctx context.Context, filter *sqln.Filters) (model.ProductQueryResponse, error) {
    // Base query DEVE ter WHERE (ou WHERE 1=1) para NewQueryBuild poder anexar AND (...)
    query := "SELECT " + productQuerySelectFields + " FROM product WHERE 1=1"

    return sqln.FindWithFilter[model.ProductQuery](ctx,
        sqln.NewQueryBuild(query, filter),
    ).WithPage(
        sqln.NewPageRequestFilter(filter),
    ).PagedList()
}
```

**4. service — GetByDynamicQuery:**
```go
// Interface
GetByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.ProductQueryResponse, errs.AppError)

// Implementação — sem validação, sem transformação
func (s *productService) GetByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.ProductQueryResponse, errs.AppError) {
    result, err := s.repo.FindByDynamicQuery(ctx, filters)
    if err != nil {
        return nil, ErrProductQuery.Wrap(err)  // mesmo ErrXxxQuery do GetByFilter
    }
    return result, errs.AppError{}
}
```

### API completa de filtro dinâmico

**Construtores de filtro (além de `sqln.Eq` e `sqln.Contains`):**
```go
// Comparação
sqln.Eq, sqln.NotEqual
sqln.Less, sqln.LessOrEqual, sqln.Greater, sqln.GreaterOrEqual

// Membership
sqln.In, sqln.NotIn

// Texto (case-insensitive via dialeto ativo)
sqln.Contains, sqln.NotContains   // ILIKE no PostgreSQL
sqln.Like, sqln.NotLike           // LIKE literal (case-sensitive)

// Range (Between): passar slice de 2 ou string "RFC3339|RFC3339"
sqln.Between

// Null check (sem value)
sqln.IsNull, sqln.IsNotNull

// Separadores lógicos (nós de filtro, não apenas constantes)
sqln.AND()   // *Filter com LogicalOperator = "and"
sqln.OR()    // *Filter com LogicalOperator = "or"
```

**Estrutura de `sqln.Filters` (body da request):**
```go
type Filters struct {
    Tenant  int32         // injetado pelo handler (JWT), nunca do body
    Params  *FilterParams  // paginação e ordenação
    Filters []*Filter      // condições enviadas pelo cliente
}

type FilterParams struct {
    Page          uint16 // 0-indexed
    Limit         uint16 // 0 → default 15
    SortField     string // campo de ordenação
    SortDirection string // "ASC" | "DESC"
}
```

**`sqln.NewQueryBuild` — comportamento crítico:**
- Produz PostgreSQL-style (`$1, $2, ...`). Para outros bancos: `sqln.NewQueryBuildWithDialect(query, filter, dialect)`
- Se `filters.Filters` é vazio → retorna `{Query: query}` sem alteração
- Se não-vazio → **anexa `AND ( conditions )` ao final da query base**
- **A query base DEVE terminar com uma cláusula `WHERE`** (ou `WHERE 1=1` quando não há condição fixa) para o `AND` ser SQL válido

**`sqln.NewPageRequestFilter` — paginação a partir de `Filters.Params`:**
- Extrai `Page`, `Limit`, `SortField`, `SortDirection` de `filter.Params`
- Aplica defaults: page=0, limit=15, sortField="id", sortDirection="ASC"
- Diferença de `sqln.NewPageRequest`: aceita `*Filters` direto, sem parâmetros separados

### Regras do filtro dinâmico

- `query_dto.go` é **arquivo separado** de `dto.go` dentro de `model/` — não misturar
- `{Context}Query` struct usa tags `db:""` (read model), não `gofi:""` (entity)
- `queryMapping.Validate(filters)` deve ser chamado no handler **antes** do service
- Fallback de filtro default (`filters.Add(...)`) fica no handler quando `len(filters.Filters) == 0`
- `filters.Tenant` é injetado pelo handler (do JWT) — nunca vem do body do cliente
- Service recebe `*sqln.Filters` e repassa para o repository sem transformar
- Repository usa `FindWithFilter` + `NewQueryBuild` + `NewPageRequestFilter` — **não** `FindFromCriteria`
- Query base do repository **deve ter `WHERE` (ou `WHERE 1=1`)** antes de `NewQueryBuild`
- Usar `productQuerySelectFields` (constante separada de `productSelectFields`) quando o read model de query difere da entidade

## Comportamento de FindByID com nil

```go
person, err := sqln.FindFromCriteria[model.Person](ctx, q).Execute()
// Se não encontrado: person == nil, err == nil
// Se erro de scan: person == nil, err != nil
// Se encontrado: person != nil, err == nil
```

Service deve checar `person == nil` para retornar not found.

## Retorno de Tipos Primitivos — `(*T, error)` direto

Quando o repository consulta **um único valor primitivo** (existência, flag), devolva o retorno nativo de `FindFromCriteria[T](...).Execute()`: `(*T, error)`. Nunca converta manualmente para `(T, error)` — a semântica `nil` vs não-nil já é o que o SDK oferece e alinha com `FindByID`.

```go
// Interface — primitivo como ponteiro
ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error)

// Implementação — sem conversão manual
func (r *userRepository) ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error) {
    return sqln.FindFromCriteria[bool](ctx,
        criteria.From(`"user"`, "u").
            Select("u.id").
            Where(criteria.Eq("u.email", email)).
            Where(criteria.Eq("u.tenant_id", tenantID)),
    ).Execute()
}

// Consumo no service — o valor apontado é irrelevante, checa presença
exists, err := s.repo.ExistsByEmailAndTenant(ctx, email, tenantID)
if err != nil  { return ErrUserCreate.Wrap(err) }
if exists != nil { return ErrUserConflict.New() }
```

### Anti-padrão

```go
// NÃO FAZER — duplica a checagem que o SDK já oferece
result, err := sqln.FindFromCriteria[bool](ctx, q).Execute()
if err != nil { return false, err }
return result != nil, nil
```

### Escopo

- `Exists*` → `(*bool, error)`
- Qualquer consulta de **presença** de um único valor primitivo via `FindFromCriteria[T]`
- **Não** se aplica a listas (`[]T`, `*sqln.Page[T]`) nem a contagens numéricas reais

Referência completa: `.claude/sdk/<lang>/knowledge/repository-primitive-return.md` e `.claude/sdk/<lang>/boilerplates/repository.md` (seção "Consulta de Presença").
