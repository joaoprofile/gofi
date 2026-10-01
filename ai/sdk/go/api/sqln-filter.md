# sqln/filter

`import "github.com/joaoprofile/gofi-sdk-go/sqln/filter"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

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
	Contains    = driver.Contains
	NotContains = driver.NotContains
	Like        = driver.Like
	NotLike     = driver.NotLike
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
	And = driver.And
	Or  = driver.Or
)
```

```go
const Between = driver.Between
```

## Variáveis

```go
var (
	Equality = []string{Eq, NotEqual, In, NotIn, IsNull, IsNotNull}
	Range    = []string{Eq, NotEqual, Less, LessOrEqual, Greater, GreaterOrEqual, Between, IsNull, IsNotNull}
	Text     = []string{Eq, NotEqual, In, NotIn, Contains, NotContains, Like, NotLike, IsNull, IsNotNull}
)
```

Operator sets for Field.Ops.

```go
var ErrInvalidFilter = errors.New("sqln/filter: invalid filter")
```

ErrInvalidFilter is wrapped by Build for every rejected filter or sort.

## Funções

### filter.ContainsNotAllowedValue

```go
func ContainsNotAllowedValue(input string) bool
```

ContainsNotAllowedValue reports whether a string contains SQL injection keywords.
Uses word-boundary matching to avoid false positives (e.g. "EXECUTOR" does not match "EXEC").
Secondary defense only — values are always bound as parameters, never interpolated.

## filter.Field

```go
type Field struct {
	// Column is the SQL column expression, e.g. "o.created_at".
	Column string `json:"-"`
	// Ops lists the accepted conditions; empty accepts every operator.
	Ops []string `json:"ops,omitempty"`
	// Sortable allows the name as a sort field.
	Sortable bool `json:"sortable,omitempty"`

	// UI metadata, passed through untouched.
	Label      string `json:"label,omitempty"`
	FilterType string `json:"filterType,omitempty"`
	SearchType string `json:"searchType,omitempty"`
	Content    any    `json:"content,omitempty"`
}
```

Field describes one filterable name. Column is never serialized, so the
mapping can be sent to a frontend to build filter screens.

## filter.Filter

```go
type Filter struct {
	Field           string `json:"field,omitempty"`
	Condition       string `json:"condition,omitempty"`
	Value           any    `json:"value,omitempty"`
	LogicalOperator string `json:"logicalOperator,omitempty"`
}
```

Domain Types

### filter.AND

```go
func AND() *Filter
```

### filter.NewFilter

```go
func NewFilter(field, condition string, value any) *Filter
```

### filter.OR

```go
func OR() *Filter
```

## filter.FilterDialect

```go
type FilterDialect = driver.FilterDialect
```

## filter.FilterParams

```go
type FilterParams struct {
	Page          uint16 `json:"page"`
	Limit         uint16 `json:"limit"`
	SortField     string `json:"sortField"`
	SortDirection string `json:"sortDirection"`
}
```

FilterParams holds pagination and sorting parameters.

## filter.Filters

```go
type Filters struct {
	Tenant  any
	Params  *FilterParams `json:"params"`
	Filters []*Filter     `json:"filters"`
}
```

Filters is the root container for a dynamic query filter request.

### filter.NewFilters

```go
func NewFilters() *Filters
```

### Filters.Add

```go
func (f *Filters) Add(filter ...*Filter) *Filters
```

## filter.FloatValue

```go
type FloatValue float64
```

## filter.IntValue

```go
type IntValue int64
```

## filter.Mapping

```go
type Mapping map[string]Field
```

Mapping is the allowlist of a dynamic query: the names an API accepts,
each bound to the column it filters. Fields outside the mapping are
rejected, so clients cannot probe columns that are never returned (for
example password_hash LIKE 'a%').

### filter.Allow

```go
func Allow(columns ...string) Mapping
```

Allow maps each column to itself with every operator and sorting enabled.
It keeps the names clients already send while adding the allowlist; move
to API names (Mapping{"created": {Column: "o.created_at"}}) when the
contract can change.

### Mapping.SortColumn

```go
func (m Mapping) SortColumn(field string) (string, error)
```

SortColumn resolves a sort field through the mapping; an empty field
returns "" (the caller's default applies).

## filter.QueryParam

```go
type QueryParam struct {
	Query  string
	Params []any
}
```

QueryParam holds the final SQL fragment and its bound parameters.

### filter.Build

```go
func Build(base string, args []any, filters *Filters, m Mapping, dialect FilterDialect) (*QueryParam, error)
```

Build appends the filters to base, which must end inside a WHERE clause,
as "base AND ( … )". Placeholders continue after args, the parameters base
already binds, and the returned Params are args followed by the filter
values. Any filter outside m, or otherwise invalid, rejects the whole set.
A nil dialect uses the active global connection.

## filter.SliceValue

```go
type SliceValue []any
```

## filter.StringValue

```go
type StringValue string
```

Value Type Wrappers

