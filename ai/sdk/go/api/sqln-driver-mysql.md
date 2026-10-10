# sqln/driver/mysql

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/mysql"` · SDK gofi/v0.2.4 (310ba1a)

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

DSN builds a go-sql-driver/mysql DSN: user:pass@tcp(host:port)/dbname?tls=….
The database name is path-escaped and the driver splits the DSN at the
last '@' and '/', so the password may hold any character. The tls
parameter follows s.EffectiveSSLMode(): disable → false, prefer →
preferred, require → skip-verify, anything else → true (verify-full).

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

ParseError hides the server message behind a generic one (see connection.Error).

### Driver.ValidateSettings

```go
func (Driver) ValidateSettings(s connection.Settings) error
```

ValidateSettings rejects what the DSN cannot express. A custom CA or client
certificate needs a tls.Config registered with mysql.RegisterTLSConfig and
a hand-built DSN with tls=<name>.

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

