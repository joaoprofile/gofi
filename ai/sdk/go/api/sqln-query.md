# sqln/query

`import "github.com/joaoprofile/gofi-sdk-go/sqln/query"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## query.Query

```go
type Query interface {
	FetchRows(ctx context.Context, dbConn *sql.DB, query string, args ...any) (*sql.Rows, error)
	FetchRow(ctx context.Context, dbConn *sql.DB, query string, args ...any) *sql.Row
	Execute(ctx context.Context, dbConn *sql.DB, query string, args ...any) *sql.Row
}
```

### query.NewQuery

```go
func NewQuery() Query
```

## query.SQLQuery

```go
type SQLQuery struct {
	Query  string
	Params []any
}
```

