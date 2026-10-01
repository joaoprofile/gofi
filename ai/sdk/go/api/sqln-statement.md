# sqln/statement

`import "github.com/joaoprofile/gofi-sdk-go/sqln/statement"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## statement.Statement

```go
type Statement interface {
	// Execute runs query directly (no server-side prepare, PgBouncer-safe) and
	// returns the result, e.g. for RowsAffected in optimistic locking.
	Execute(ctx context.Context, query string, args ...any) (sql.Result, error)
	Prepare(ctx context.Context, query string) (*sql.Stmt, error)
	// QueryRow returns an error instead of panicking when the connection is
	// missing or the query is empty.
	QueryRow(ctx context.Context, query string, args ...any) (*sql.Row, error)
}
```

Statement runs write statements and single-row queries on the global
connection, joining the transaction carried by ctx when there is one.

### statement.NewStatement

```go
func NewStatement() Statement
```

NewStatement runs on the global connection.

### statement.NewWithConnection

```go
func NewWithConnection(conn *connection.Connection) Statement
```

NewWithConnection runs on conn, for services with more than one database.

