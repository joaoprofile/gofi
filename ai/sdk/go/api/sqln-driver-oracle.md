# sqln/driver/oracle

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/oracle"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## oracle.Driver

```go
type Driver struct{}
```

Oracle driver. To enable it, blank-import this package:

	import _ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/oracle"

Requires godror or go-oci8 in go.mod.

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a go-ora URL, oracle://user:pass@host:port/service, with every
part escaped. TLS follows s.EffectiveSSLMode(): disable → none, require →
SSL=true without verification, anything else → SSL=true with
"SSL VERIFY"=true (verify-full). Custom CAs go in a go-ora wallet.

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

## oracle.OracleDialect

```go
type OracleDialect struct{}
```

### OracleDialect.BuildCount

```go
func (OracleDialect) BuildCount(query string) string
```

### OracleDialect.BuildPagination

```go
func (OracleDialect) BuildPagination(query string, order string, limit uint16, offset uint64) string
```

### OracleDialect.Like

```go
func (OracleDialect) Like(field string, param string) string
```

### OracleDialect.NotLike

```go
func (OracleDialect) NotLike(field string, param string) string
```

### OracleDialect.Param

```go
func (OracleDialect) Param(index int) string
```

