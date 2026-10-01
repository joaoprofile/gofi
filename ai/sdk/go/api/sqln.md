# sqln

`import "github.com/gofi-labs/gofi-sdk-go/sqln"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	DefaultPage          = pagination.DefaultPage
	DefaultLimit         = pagination.DefaultLimit
	DefaultSortField     = pagination.DefaultSortField
	DefaultSortDirection = pagination.DefaultSortDirection
)
```

```go
const (
	MsgTransactionIsolationIgnored = "transaction isolation only uses the first parameter, others are ignored"
	ErrExecuteTransaction          = "error when executing transaction error: %w"
	ErrTransactionRollback         = "error when executing transaction rollback: %v, original error: %w"
	ErrTransactionCommit           = "could not commit transaction: %w"
	ErrTransactionStart            = "could not start database transaction: %v"
)
```

```go
const (
	ErrDatabaseNotInitialized = connection.ErrDatabaseNotInitialized
	ErrMsgQueryIsEmpty        = connection.ErrQueryIsEmpty
)
```

Re-exported from connection for backward compatibility.

```go
const (
	ErrMsgFailedConnect        string = "failed to connect to PostgreSQL: %w"
	ErrMsgFailedConnectionPing string = "failed to connect to PostgreSQL ping: %w"
	ErrMsgMigration            string = "an error occurred when validate database migrations: %v"
	ErrPageIsEmpty             string = "page is empty, please create a pageable query"
	ErrMsgStmClose             string = "failed to close statement: %v\n"
)
```

```go
const (
	Eq             = driver.Eq
	NotEqual       = driver.NotEqual
	Less           = driver.Less
	LessOrEqual    = driver.LessOrEqual
	Greater        = driver.Greater
	GreaterOrEqual = driver.GreaterOrEqual
)
```

```go
const (
	In    = driver.In
	NotIn = driver.NotIn
)
```

```go
const (
	// Contains performs a case-insensitive substring match via the active dialect.
	// PostgreSQL: ILIKE   MySQL / SQL Server / Oracle: LIKE
	Contains = driver.Contains

	// NotContains performs a case-insensitive negative substring match via the active dialect.
	// PostgreSQL: NOT ILIKE   Others: NOT LIKE
	NotContains = driver.NotContains

	// Like performs a literal case-sensitive LIKE on all databases.
	Like = driver.Like

	// NotLike performs a literal case-sensitive NOT LIKE on all databases.
	NotLike = driver.NotLike
)
```

```go
const (
	IsNull    = driver.IsNull
	IsNotNull = driver.IsNotNull
)
```

```go
const (
	Or  = driver.Or
	And = driver.And
)
```

```go
const Between = driver.Between
```

```go
const SqlTxContextKey = connection.SqlTxContextKey
```

SqlTxContextKey re-exported from connection for backward compatibility.
transaction.Execute stores the active *sql.Tx under this key.

## Variáveis

```go
var (
	Equality = filter.Equality
	Range    = filter.Range
	Text     = filter.Text
)
```

Operator sets for FilterField.Ops.

```go
var ErrInvalidFilter = filter.ErrInvalidFilter
```

ErrInvalidFilter is wrapped by BuildQuery for each rejected filter or sort.

## Funções

### sqln.BuildClause

```go
func BuildClause(predicates []Predicate, dialect driver.FilterDialect) (string, []any)
```

BuildClause compiles a predicate slice into a SQL WHERE fragment and its bound parameters.
The fragment does NOT include the WHERE keyword — designed for embedding into an existing
base query (e.g., base + " AND ( " + clause + " )").
Adjacent predicates without an explicit And()/Or() connector are implicitly joined with AND.

### sqln.CloseRedis

```go
func CloseRedis() error
```

CloseRedis closes the shared cache client created by InstanceRedis.

### sqln.Find

```go
func Find[T any](ctx context.Context, query string, params ...any) *manager[T]
```

### sqln.FindFromCriteria

```go
func FindFromCriteria[T any](ctx context.Context, q *criteria.Query) *manager[T]
```

### sqln.FindWithFilter

```go
func FindWithFilter[T any](ctx context.Context, params *QueryParam) *manager[T]
```

### sqln.InstanceRedis

```go
func InstanceRedis() redis.UniversalClient
```

InstanceRedis re-exported from cache/ for backward compatibility.

### sqln.NewCacheRedis

```go
func NewCacheRedis()
```

NewCacheRedis re-exported from cache/ for backward compatibility.

### sqln.NewCustomQuery

```go
func NewCustomQuery[T any](ctx context.Context, query string, params ...any) *manager[T]
```

### sqln.PingRedis

```go
func PingRedis(ctx context.Context) error
```

PingRedis checks the shared cache client.

## sqln.Cache

```go
type Cache[T any] = cache.Cache[T]
```

Cache type alias for backward compatibility.

### sqln.NewCache

```go
func NewCache[T any](name string, ttl time.Duration) *Cache[T]
```

NewCache re-exported from cache/ for backward compatibility.

## sqln.CriteriaOrder

```go
type CriteriaOrder = criteria.Order
```

CriteriaOrder defines a sort expression for a criteria query.

### sqln.Asc

```go
func Asc(field string) CriteriaOrder
```

Asc creates an ascending ORDER BY expression for criteria queries.

### sqln.Desc

```go
func Desc(field string) CriteriaOrder
```

Desc creates a descending ORDER BY expression for criteria queries.

## sqln.CriteriaQuery

```go
type CriteriaQuery = criteria.Query
```

CriteriaQuery is the declarative SQL query builder.
Use CriteriaFrom() as the entry point, or import the criteria sub-package directly

### sqln.CriteriaFrom

```go
func CriteriaFrom(table, alias string) *CriteriaQuery
```

Entry Point

## sqln.Filter

```go
type Filter = filter.Filter
```

### sqln.AND

```go
func AND() *Filter
```

### sqln.NewFilter

```go
func NewFilter(field, condition string, value any) *Filter
```

### sqln.OR

```go
func OR() *Filter
```

## sqln.FilterDialect

```go
type FilterDialect = driver.FilterDialect
```

FilterDialect is the SQL generation interface required by the filter engine.
Sourced from the driver package — the single authoritative definition.

## sqln.FilterField

```go
type FilterField = filter.Field
```

## sqln.FilterMapping

```go
type FilterMapping = filter.Mapping
```

FilterMapping is the allowlist of names a dynamic query accepts, each bound
to its column; FilterField describes one name (see filter.Mapping).

### sqln.AllowColumns

```go
func AllowColumns(columns ...string) FilterMapping
```

AllowColumns maps each column to itself (see filter.Allow).

## sqln.FilterParams

```go
type FilterParams = filter.FilterParams
```

## sqln.Filters

```go
type Filters = filter.Filters
```

### sqln.NewFilters

```go
func NewFilters() *Filters
```

## sqln.FloatValue

```go
type FloatValue = filter.FloatValue
```

## sqln.IntValue

```go
type IntValue = filter.IntValue
```

## sqln.Manager

```go
type Manager[T any] interface {
	ExecuteListQuery(db *sql.DB) ([]T, error)
	ExecuteUniqueResultQuery(db *sql.DB) (*T, error)
	ExecutePagedQuery(db *sql.DB) (*Page[T], error)
}
```

Manager defines a read-only query interface that can be mocked in tests.

## sqln.Page

```go
type Page[T any] = pagination.Page[T]
```

## sqln.PageRequest

```go
type PageRequest = pagination.PageRequest
```

### sqln.NewPageRequest

```go
func NewPageRequest(page uint16, limit uint16, order []Sort) *PageRequest
```

### sqln.NewPageRequestFilter

```go
func NewPageRequestFilter(f *filter.Filters, m filter.Mapping) (*PageRequest, error)
```

NewPageRequestFilter builds the page from the request parameters; the sort
field is resolved through m and must be Sortable.

## sqln.Predicate

```go
type Predicate = criteria.Predicate
```

Predicate represents a WHERE/HAVING condition or a logical connector (AND/OR).

## sqln.Query

```go
type Query = query.Query
```

### sqln.NewQuery

```go
func NewQuery() Query
```

## sqln.QueryParam

```go
type QueryParam = filter.QueryParam
```

### sqln.BuildQuery

```go
func BuildQuery(base string, args []any, f *Filters, m FilterMapping, dialect FilterDialect) (*QueryParam, error)
```

BuildQuery appends the filters to base through the mapping; placeholders
continue after args. A nil dialect uses the active connection (see filter.Build).

## sqln.SQLQuery

```go
type SQLQuery = query.SQLQuery
```

## sqln.Sort

```go
type Sort = pagination.Sort
```

### sqln.NewSort

```go
func NewSort(field string, direction SortDirection) Sort
```

## sqln.SortDirection

```go
type SortDirection = pagination.SortDirection
```

SortDirection re-exported from pagination for backward compatibility.

```go
const (
	ASC  SortDirection = pagination.ASC
	DESC SortDirection = pagination.DESC
)
```

## sqln.Statement

```go
type Statement = statement.Statement
```

### sqln.NewStatement

```go
func NewStatement() Statement
```

## sqln.StringValue

```go
type StringValue = filter.StringValue
```

Value type wrappers for type-safe parameter binding in tests and custom scan logic.

## sqln.Transaction

```go
type Transaction = transaction.Transaction
```

### sqln.NewTransaction

```go
func NewTransaction(isolation ...sql.IsolationLevel) Transaction
```

