# sqln/mapping

`import "github.com/gofi-labs/gofi-sdk-go/sqln/mapping"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### mapping.Each

```go
func Each[T any](rows *sql.Rows) iter.Seq2[T, error]
```

Each streams rows as T values and closes rows when done or when the
consumer stops early.

### mapping.GetContentList

```go
func GetContentList[T any](rows *sql.Rows) ([]T, error)
```

GetContentList reads all rows and scans them into []T using `db` tags.
Columns are matched by name when they cover every tagged field (extra columns
are ignored); otherwise fields are scanned by position, as before.

### mapping.GetMappedCols

```go
func GetMappedCols(model any) []any
```

GetMappedCols returns the addresses of fields tagged with `db` for use in
rows.Scan. The struct's leaf layout is cached per type — the first call for
a given type builds the plan, subsequent calls reuse it.

Nested structs tagged with `db` are expanded recursively — their `db`-tagged
sub-fields become scan columns. time.Time and types implementing sql.Scanner
are treated as primitive values.

### mapping.GetUnique

```go
func GetUnique[T any](rows *sql.Rows) (*T, error)
```

GetUnique scans the first row into a *T with the same rules as GetContentList.
Returns (nil, nil) when there are no rows.

### mapping.IsSimpleType

```go
func IsSimpleType(value any) bool
```

IsSimpleType returns true if the value is neither a struct nor a pointer.

