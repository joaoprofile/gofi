# sqln/connection

`import "github.com/joaoprofile/gofi-sdk-go/sqln/connection"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	ErrDriverNotRegistered = "database driver not registered"
	ErrPingFailed          = "database ping failed"
)
```

```go
const (
	ErrDatabaseNotInitialized = "database connection not initialized"
	ErrQueryIsEmpty           = "query is empty"
)
```

Shared error message constants used across sqln subpackages.

```go
const SqlTxContextKey txContextKey = "sqlTxContext"
```

SqlTxContextKey is the context key used to propagate *sql.Tx within a request.
transaction.Execute stores the active transaction under this key; Statement
and Query read it to participate in the same transaction automatically.
With several databases use TxFor, which also knows which pool owns it.

## Variáveis

```go
var ErrPasswordFuncUnsupported = errors.New("sqln: Config.Password is not supported by this driver")
```

ErrPasswordFuncUnsupported is returned by drivers that cannot use Config.Password.

## Funções

### connection.AsPgError

```go
func AsPgError(err error) (*pgconn.PgError, bool)
```

AsPgError extracts the PostgreSQL error from err's chain.

### connection.DB

```go
func DB() (*sql.DB, error)
```

### connection.Dialect

```go
func Dialect() driver.Dialect
```

Dialect returns the SQL dialect of the active global connection.
Returns nil if no connection has been established yet.

### connection.IsRetryable

```go
func IsRetryable(err error) bool
```

IsRetryable reports whether err is a transient transaction conflict that
succeeds when the whole transaction runs again. It recognises errors that
expose SQLState() (pgx, lib/pq).

### connection.LogPostgresError

```go
func LogPostgresError(err error)
```

LogPostgresError logs database errors in structured form, with code,
table and constraint for PostgreSQL errors.

### connection.LogQueryDuration

```go
func LogQueryDuration(start time.Time, query string)
```

LogQueryDuration records how long a query took.
Queries above 300ms are logged as warning, below that as debug.

### connection.MustDB

```go
func MustDB() *sql.DB
```

### connection.RegisterDriver

```go
func RegisterDriver(d Driver)
```

### connection.ResetGlobalForTest

```go
func ResetGlobalForTest()
```

ResetGlobalForTest resets the global connection state.
Must only be called from tests; not safe for concurrent use.

### connection.SetGlobal

```go
func SetGlobal(conn *Connection)
```

SetGlobal sets the process-wide connection; the first call wins.

### connection.TxFor

```go
func TxFor(ctx context.Context, db *sql.DB) (*sql.Tx, bool)
```

TxFor returns the transaction ctx carries for db. A transaction stored
directly under SqlTxContextKey, without WithTx, is assumed to be db's.

### connection.TxFrom

```go
func TxFrom(ctx context.Context) (*sql.Tx, bool)
```

TxFrom returns the transaction carried by ctx (see transaction.Execute).

### connection.WithTx

```go
func WithTx(ctx context.Context, db *sql.DB, tx *sql.Tx) context.Context
```

WithTx returns ctx carrying tx opened on db. Outer transactions on other
pools stay available through TxFor.

## connection.Config

```go
type Config struct {
	Driver DriverName
	DSN    string
	// ReadDSN points at a read replica. When set, queries run by the sqln
	// query manager outside a transaction use it; writes, statements and
	// transactions always use DSN. Expect replica lag on reads.
	ReadDSN string
	Pool    PoolConfig
	// Password, when set, is called for every new physical connection and
	// overrides the DSN password: short-lived IAM tokens (RDS, Aurora) are
	// fetched as the pool grows. Supported by the postgres driver.
	Password func(ctx context.Context) (string, error)
}
```

## connection.Connection

```go
type Connection struct {
	// contains filtered or unexported fields
}
```

### connection.Global

```go
func Global() (*Connection, error)
```

### connection.NewConnection

```go
func NewConnection(cfg Config, opts ...Option) (*Connection, error)
```

### connection.NewRaw

```go
func NewRaw(db *sql.DB, d Driver) *Connection
```

NewRaw builds a Connection from an existing *sql.DB and a Dialect.
Useful when integrating with legacy code that manages the pool itself.

### Connection.Close

```go
func (c *Connection) Close() error
```

### Connection.DB

```go
func (c *Connection) DB() *sql.DB
```

DB returns the primary pool.

### Connection.Dialect

```go
func (c *Connection) Dialect() driver.Dialect
```

### Connection.ReadDB

```go
func (c *Connection) ReadDB() *sql.DB
```

ReadDB returns the replica pool, or the primary when none is configured.

## connection.ConnectionObserver

```go
type ConnectionObserver struct {
	// contains filtered or unexported fields
}
```

### connection.NewObserver

```go
func NewObserver(name string, conn *Connection) *ConnectionObserver
```

### ConnectionObserver.Close

```go
func (o *ConnectionObserver) Close()
```

## connection.Driver

```go
type Driver interface {
	Name() DriverName
	// DSN assembles the driver-specific connection string from s. Each driver
	// owns its own format (key-value for postgres, URL for sqlserver, …) so new
	// databases can be added without a central switch.
	DSN(s Settings) string
	Open(cfg Config) (*sql.DB, error)
	ParseError(err error) error
	Dialect() driver.Dialect
}
```

### connection.GetDriver

```go
func GetDriver(name DriverName) (Driver, bool)
```

## connection.DriverName

```go
type DriverName string
```

```go
const (
	DriverPostgres  DriverName = "postgres"
	DriverMySQL     DriverName = "mysql"
	DriverOracle    DriverName = "oracle"
	DriverSQLServer DriverName = "sqlserver"
)
```

## connection.Option

```go
type Option func(*options)
```

options

### connection.WithMigrations

```go
func WithMigrations(cfg migrate.Config) Option
```

## connection.PoolConfig

```go
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	MaxConnLifeTime time.Duration
	// MaxIdleTime closes connections idle for longer, returning them to the
	// database between bursts (0 keeps them until MaxConnLifeTime).
	MaxIdleTime time.Duration
}
```

### connection.DefaultPoolConfig

```go
func DefaultPoolConfig() PoolConfig
```

## connection.Querier

```go
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
```

Querier is the common surface of *sql.DB, *sql.Tx and *sql.Conn, so
repositories run the same code inside and outside a transaction.

### connection.QuerierFrom

```go
func QuerierFrom(ctx context.Context, db *sql.DB) Querier
```

QuerierFrom returns db's transaction carried by ctx, or db.

## connection.Settings

```go
type Settings struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}
```

Settings holds the structured connection parameters that a Driver assembles
into its driver-specific DSN. It decouples DSN construction from any single
configuration source: gofi's config.Database fills it from the DATABASE_*
environment variables, but callers can build it by hand just as well.

