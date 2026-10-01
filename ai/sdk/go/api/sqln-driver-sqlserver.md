# sqln/driver/sqlserver

`import "github.com/gofi-labs/gofi-sdk-go/sqln/driver/sqlserver"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## sqlserver.Driver

```go
type Driver struct{}
```

SQL Server driver. To enable it, blank-import this package:

	import _ "github.com/gofi-labs/gofi-sdk-go/sqln/driver/sqlserver"

Requires github.com/denisenkom/go-mssqldb in go.mod:

	go get github.com/denisenkom/go-mssqldb

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a go-mssqldb URL: sqlserver://user:pass@host:port?database=dbname.

### Driver.Dialect

```go
func (Driver) Dialect() sqln_driver.Dialect
```

### Driver.Name

```go
func (Driver) Name() connection.DriverName
```

### Driver.Open

```go
func (Driver) Open(cfg connection.Config) (*sql.DB, error)
```

### Driver.ParseError

```go
func (Driver) ParseError(err error) error
```

## sqlserver.SQLServerDialect

```go
type SQLServerDialect struct{}
```

### SQLServerDialect.BuildCount

```go
func (SQLServerDialect) BuildCount(query string) string
```

### SQLServerDialect.BuildPagination

```go
func (SQLServerDialect) BuildPagination(query string, order string, limit uint16, offset uint64) string
```

### SQLServerDialect.Like

```go
func (SQLServerDialect) Like(field string, param string) string
```

### SQLServerDialect.NotLike

```go
func (SQLServerDialect) NotLike(field string, param string) string
```

### SQLServerDialect.Param

```go
func (SQLServerDialect) Param(index int) string
```

