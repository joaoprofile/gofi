# sqln/driver/sqlserver

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/sqlserver"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## sqlserver.Driver

```go
type Driver struct{}
```

SQL Server driver. To enable it, blank-import this package:

	import _ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/sqlserver"

Requires github.com/denisenkom/go-mssqldb in go.mod:

	go get github.com/denisenkom/go-mssqldb

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a go-mssqldb URL, sqlserver://user:pass@host:port?database=dbname,
with every part escaped. Encryption follows s.EffectiveSSLMode(): disable →
encrypt=disable, require → encrypt=true with TrustServerCertificate=true,
anything else → encrypt=true verifying the certificate (verify-full);
SSLRootCert becomes the certificate parameter.

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

ValidateSettings rejects what the DSN cannot express.

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

### SQLServerDialect.LikeWildcards

```go
func (SQLServerDialect) LikeWildcards() string
```

LikeWildcards reports '[', which opens a character class in T-SQL LIKE.

### SQLServerDialect.NotLike

```go
func (SQLServerDialect) NotLike(field string, param string) string
```

### SQLServerDialect.Param

```go
func (SQLServerDialect) Param(index int) string
```

