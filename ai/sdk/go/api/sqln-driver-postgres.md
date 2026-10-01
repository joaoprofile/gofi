# sqln/driver/postgres

`import "github.com/gofi-labs/gofi-sdk-go/sqln/driver/postgres"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package postgres registers the PostgreSQL driver, built on pgx/v5.

## postgres.Driver

```go
type Driver struct{}
```

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a key-value connection string. An empty SSLMode defaults to
"disable" so the resulting DSN is always valid.

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

### Driver.ParseError

```go
func (Driver) ParseError(err error) error
```

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

