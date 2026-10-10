# sqln/driver/postgres

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package postgres registers the PostgreSQL driver, built on pgx/v5.

## Variáveis

```go
var ErrPasswordFuncNeedsVerifiedTLS = errors.New("postgres: Config.Password requires sslmode verify-ca or verify-full")
```

ErrPasswordFuncNeedsVerifiedTLS is returned by Open when Config.Password
(an IAM token) would travel without a verified TLS connection.

## postgres.Driver

```go
type Driver struct{}
```

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a key-value connection string with every value quoted, so no
value can add keys. sslmode is s.EffectiveSSLMode(): verify-full unless the
host is local. StatementTimeout becomes the statement_timeout runtime
parameter (not accepted by PgBouncer in transaction mode).

### Driver.Dialect

```go
func (Driver) Dialect() driver.Dialect
```

### Driver.Name

```go
func (Driver) Name() connection.DriverName
```

### Driver.Open

```go
func (Driver) Open(cfg connection.Config) (*sql.DB, error)
```

Open returns a pgx-backed *sql.DB. Unless the DSN sets
default_query_exec_mode, queries use cache_describe: one round trip without
named prepared statements, so PgBouncer in transaction mode keeps working.
With Config.Password the DSN must verify the server certificate: an IAM
token sent to an unverified server can be replayed by whoever receives it.

### Driver.ParseError

```go
func (Driver) ParseError(err error) error
```

ParseError hides the server message behind a generic one (see connection.Error).

## postgres.MigrateDriver

```go
type MigrateDriver struct{}
```

### MigrateDriver.Instance

```go
func (MigrateDriver) Instance(db *sql.DB) (database.Driver, error)
```

### MigrateDriver.Name

```go
func (MigrateDriver) Name() string
```

## postgres.PostgresDialect

```go
type PostgresDialect struct{}
```

### PostgresDialect.ArrayMembership

```go
func (PostgresDialect) ArrayMembership(field, param string, negate bool) string
```

ArrayMembership binds IN lists as one array: the SQL text is the same for
any list length. <> ALL keeps NOT IN's NULL semantics.

### PostgresDialect.BuildCount

```go
func (PostgresDialect) BuildCount(query string) string
```

COUNT(*) rather than COUNT(tb.*): both count the same rows (an all-NULL row
included), but naming the composite row forces the planner to materialize it,
and it then gives up eliminating the unique-key LEFT JOINs nothing projects.

### PostgresDialect.BuildPagination

```go
func (PostgresDialect) BuildPagination(query string, order string, limit uint16, offset uint64) string
```

### PostgresDialect.Like

```go
func (PostgresDialect) Like(field string, param string) string
```

### PostgresDialect.NotLike

```go
func (PostgresDialect) NotLike(field string, param string) string
```

### PostgresDialect.Param

```go
func (PostgresDialect) Param(index int) string
```

