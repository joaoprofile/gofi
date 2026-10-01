# sqln/driver

`import "github.com/gofi-labs/gofi-sdk-go/sqln/driver"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	Eq             = "="
	NotEqual       = "!="
	Less           = "<"
	LessOrEqual    = "<="
	Greater        = ">"
	GreaterOrEqual = ">="
)
```

Comparison operators are valid for all databases.

```go
const (
	In    = "IN"
	NotIn = "NOT IN"
)
```

Membership operators test whether a value belongs to a set.

```go
const (
	Contains    = "LIKE"     // case-insensitive via dialect: ILIKE (PG) / LIKE (others)
	NotContains = "NOT LIKE" // case-insensitive NOT via dialect: NOT ILIKE (PG) / NOT LIKE (others)
	Like        = "~~"       // literal case-sensitive LIKE, all databases
	NotLike     = "!~~"      // literal case-sensitive NOT LIKE, all databases
)
```

```go
const (
	IsNull    = "IS NULL"
	IsNotNull = "IS NOT NULL"
)
```

Null Check Operators─
Null check operators do not require a bound value.

```go
const (
	IsTrue  = "IS TRUE"
	IsFalse = "IS FALSE"
)
```

Boolean Check Operators
Boolean check operators do not require a bound value.

```go
const (
	And = "AND"
	Or  = "OR"
)
```

Logical operators separate predicates within a filter list.

```go
const Between = "BETWEEN"
```

Between accepts:
  - a "startRFC3339|endRFC3339" string
  - a []time.Time{start, end}
  - a []any{start, end} where elements are time.Time

```go
const Group = "__group__"
```

Group Operator — internal sentinel used by Group predicate.
Never emitted verbatim; the builder wraps the inner clause in parentheses.

## Variáveis

```go
var AllowedOperators = map[string]struct{}{
	Eq: {}, NotEqual: {}, Less: {}, LessOrEqual: {}, Greater: {}, GreaterOrEqual: {},
	In: {}, NotIn: {},
	Contains: {}, NotContains: {},
	Like: {}, NotLike: {},
	Between: {},
	IsNull:  {}, IsNotNull: {},
	IsTrue: {}, IsFalse: {},
	Group: {},
}
```

AllowedOperators is the authoritative set of valid filter operator strings.
Used by the filter and criteria engines to reject unknown or injected operators.

## Funções

### driver.IsIdentifier

```go
func IsIdentifier(s string) bool
```

IsIdentifier reports whether s is a column reference such as col, t.col, "Col" or schema.t.col.

### driver.IsSortDirection

```go
func IsSortDirection(d string) bool
```

IsSortDirection reports whether d is ASC/DESC with an optional NULLS FIRST/LAST.

### driver.SortDirection

```go
func SortDirection(d string) string
```

SortDirection normalizes d; anything that is not a valid direction falls back to ASC.

## driver.ArrayDialect

```go
type ArrayDialect interface {
	// ArrayMembership renders field IN / NOT IN for an array placeholder.
	ArrayMembership(field, param string, negate bool) string
}
```

ArrayDialect is implemented by dialects that bind a whole slice as one
array parameter, so IN lists keep the same SQL text for any length.

## driver.Dialect

```go
type Dialect interface {
	FilterDialect

	// offset is uint64 because it is page*limit: two uint16 multiplied overflow
	// past 65535 and silently wrap to a different page.
	BuildPagination(query string, order string, limit uint16, offset uint64) string

	BuildCount(query string) string
}
```

## driver.FilterDialect

```go
type FilterDialect interface {
	Param(index int) string
	Like(field string, param string) string
	NotLike(field string, param string) string
}
```

