# sqln/driver/mysql

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/mysql"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## mysql.Driver

```go
type Driver struct{}
```

MySQL driver. To enable it, blank-import this package:

	import _ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/mysql"

Requires the go-sql-driver/mysql driver in go.mod:

	go get github.com/go-sql-driver/mysql

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a go-sql-driver/mysql DSN: user:pass@tcp(host:port)/dbname.

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

## mysql.MySQLDialect

```go
type MySQLDialect struct{}
```

### MySQLDialect.BuildCount

```go
func (MySQLDialect) BuildCount(query string) string
```

### MySQLDialect.BuildPagination

```go
func (MySQLDialect) BuildPagination(query string, order string, limit uint16, offset uint64) string
```

### MySQLDialect.Like

```go
func (MySQLDialect) Like(field string, param string) string
```

### MySQLDialect.NotLike

```go
func (MySQLDialect) NotLike(field string, param string) string
```

### MySQLDialect.Param

```go
func (MySQLDialect) Param(_ int) string
```

