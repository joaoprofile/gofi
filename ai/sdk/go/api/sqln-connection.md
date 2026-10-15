# sqln/connection

`import "github.com/joaoprofile/gofi-sdk-go/sqln/connection"` · SDK gofi/v0.2.6 (3df9d20)

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
const (
	SQLStateNotNullViolation    = "23502"
	SQLStateForeignKeyViolation = "23503"
	SQLStateUniqueViolation     = "23505"
	SQLStateCheckViolation      = "23514"
)
```

SQLSTATE codes of the integrity violations repositories translate into
domain errors.

```go
const (
	SSLDisable    = "disable"     // plaintext
	SSLAllow      = "allow"       // plaintext, TLS only if the server demands it (postgres)
	SSLPrefer     = "prefer"      // TLS when the server offers it, else plaintext
	SSLRequire    = "require"     // TLS without verifying the server identity
	SSLVerifyCA   = "verify-ca"   // TLS, certificate chain verified
	SSLVerifyFull = "verify-full" // TLS, certificate chain and host name verified
)
```

SSL modes accepted by Settings.SSLMode. They follow libpq's names; each
driver maps them to its own options and rejects the ones it cannot honour.

```go
const DefaultQueryTimeout = 30 * time.Second
```

DefaultQueryTimeout bounds queries when Config.QueryTimeout is zero.

```go
const SqlTxContextKey txContextKey = "sqlTxContext"
```

SqlTxContextKey is the context key used to propagate *sql.Tx within a request.
transaction.Execute stores the active transaction under this key; Statement
and Query read it to participate in the same transaction automatically.
With several databases use TxFor, which also knows which pool owns it.

## Variáveis

```go
var ErrInvalidSettings = errors.New("sqln: invalid connection settings")
```

ErrInvalidSettings is wrapped by every Settings validation failure.

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

### connection.BuildDSN

```go
func BuildDSN(d Driver, s Settings) (string, error)
```

BuildDSN validates s, generically and through d when it implements
SettingsValidator, and returns d's DSN for it.

### connection.Cause

```go
func Cause(err error) error
```

Cause returns the driver error behind err's *Error, for logs; other
errors are returned as is.

### connection.ConstraintName

```go
func ConstraintName(err error) string
```

ConstraintName returns the constraint named by the Postgres error in err's
chain, or "".

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

### connection.FormatSQL

```go
func FormatSQL(query string) string
```

FormatSQL lays out a statement for reading: one clause per line, the
SELECT list one column per line, AND/OR of WHERE and HAVING on their own
lines, JOINs indented, and subqueries indented inside their parentheses.
Function-call parentheses stay inline. It only changes whitespace outside
literals and quoted identifiers, so the statement means the same; text
that is not valid UTF-8 is returned as is.

### connection.IsCheckViolation

```go
func IsCheckViolation(err error, constraint ...string) bool
```

IsCheckViolation reports a check violation (23514), optionally only on the
given constraint.

### connection.IsForeignKeyViolation

```go
func IsForeignKeyViolation(err error, constraint ...string) bool
```

IsForeignKeyViolation reports a foreign key violation (23503), optionally
only on the given constraint.

### connection.IsNotNullViolation

```go
func IsNotNullViolation(err error) bool
```

IsNotNullViolation reports a not-null violation (23502).

### connection.IsRetryable

```go
func IsRetryable(err error) bool
```

IsRetryable reports whether err is a transient transaction conflict that
succeeds when the whole transaction runs again. It recognises errors that
expose SQLState() (pgx, lib/pq).

### connection.IsUniqueViolation

```go
func IsUniqueViolation(err error, constraint ...string) bool
```

IsUniqueViolation reports a unique violation (23505), optionally only on the
given constraint.

### connection.LogPostgresError

```go
func LogPostgresError(err error)
```

LogPostgresError logs database errors in structured form, with code,
table and constraint for PostgreSQL errors. Detail and Where quote row
values (e.g. the duplicated key), so they are logged at debug level only.

### connection.LogQueries

```go
func LogQueries(logger *slog.Logger)
```

LogQueries logs, to logger, every statement sqln sends through a Querier
— the query manager (pages and their COUNT included), statements,
transactions, the full-text checks and suggestions — as the database
receives it: the SQL, its arguments, the duration and the error. nil
stops it. The arguments may hold personal data: development only (gofi
refuses DATABASE_LOG_QUERIES in stage and prod).

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

### connection.PrintQueries

```go
func PrintQueries(w io.Writer)
```

PrintQueries writes, to w, every statement sqln runs (see LogQueries) laid
out for reading — FormatSQL, then one argument per line — framed so each
stands apart in the console; nil stops it. Keywords, parameters and
literals are colored when w is a terminal and NO_COLOR is unset. It is
independent of LogQueries, and as with it the arguments may hold personal
data: development only (gofi refuses it in stage and prod).

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

### connection.SQLState

```go
func SQLState(err error) string
```

SQLState returns the SQLSTATE code of the database error in err's chain,
through *Error and any other wrapping, or "" when there is none. Callers do
not need to know which driver sqln uses.

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

### connection.WithTimeout

```go
func WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc)
```

WithTimeout returns ctx bounded by d, unless ctx already has a deadline or
d is not positive. Always call cancel.

### connection.WithTx

```go
func WithTx(ctx context.Context, db *sql.DB, tx *sql.Tx) context.Context
```

WithTx returns ctx carrying tx opened on db. Outer transactions on other
pools stay available through TxFor.

### connection.WrapError

```go
func WrapError(op string, err error) error
```

WrapError wraps a driver error returned by op in an *Error; nil and errors
already wrapped are returned as is.

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
	// fetched as the pool grows. Supported by the postgres driver, which
	// requires sslmode verify-ca or verify-full with it.
	Password func(ctx context.Context) (string, error)
	// QueryTimeout bounds each query run by sqln whose context has no
	// deadline. Zero uses DefaultQueryTimeout; negative disables it.
	QueryTimeout time.Duration
	// FullText decides how searches on full-text indexes are checked.
	FullText FullTextConfig
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
func NewRaw(db *sql.DB, d Driver, opts ...Option) *Connection
```

NewRaw builds a Connection from an existing *sql.DB and a Dialect.
Useful when integrating with legacy code that manages the pool itself.
Its ID is random: the pool's database is unknown, so cached results are not
shared with other connections or processes.

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

### Connection.ID

```go
func (c *Connection) ID() string
```

ID identifies the database this connection reaches, so results cached for
one database are never served to another. It is a hash of the driver and
DSN (stable across processes sharing a cache); NewRaw gets a random one.

### Connection.QueryTimeout

```go
func (c *Connection) QueryTimeout() time.Duration
```

QueryTimeout is the bound sqln applies to queries whose context has no
deadline (see Config.QueryTimeout); a nil Connection has the default.

### Connection.ReadDB

```go
func (c *Connection) ReadDB() *sql.DB
```

ReadDB returns the replica pool, or the primary when none is configured.

### Connection.RequireTextIndexes

```go
func (c *Connection) RequireTextIndexes(ctx context.Context, indexes ...*fts.Index) error
```

RequireTextIndexes applies the connection's full-text policy (see
FullTextConfig) to each index not yet accepted: with fts.Verify it fails
with fts.ErrIndexMissing, fts.ErrIndexOutdated or fts.ErrServerConfig when
the index is not in the database as defined. An accepted index is not
checked again on this connection; a failure is, so applying the migration
fixes it without a restart.

### Connection.WithQueryTimeout

```go
func (c *Connection) WithQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc)
```

WithQueryTimeout bounds ctx by QueryTimeout (see WithTimeout).

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
	// databases can be added without a central switch. Every value is escaped;
	// use BuildDSN to also reject invalid settings.
	DSN(s Settings) string
	Open(cfg Config) (*sql.DB, error)
	// ParseError wraps a driver error so its message stays generic (see WrapError).
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

## connection.Error

```go
type Error struct {
	Op  string // query, exec, prepare, scan
	Err error
}
```

Error hides a database error behind a generic message: driver messages
carry table, column and constraint names and, in details, row values that
must not reach API clients. errors.As/Is still reach the driver error
(e.g. *pgconn.PgError, sql.ErrNoRows) and SQLState keeps the code.

### Error.Error

```go
func (e *Error) Error() string
```

### Error.SQLState

```go
func (e *Error) SQLState() string
```

SQLState returns the SQLSTATE code of the driver error, or "".

### Error.Unwrap

```go
func (e *Error) Unwrap() error
```

## connection.FullTextConfig

```go
type FullTextConfig struct {
	// Policy defaults to fts.Verify: each index is checked in the catalog the
	// first time it is searched, and the search fails when it is missing or
	// was built from another definition.
	Policy fts.Policy
	// Indexes are checked (or created, with fts.Ensure) when the connection
	// opens, after migrations, so a missing index fails the startup instead
	// of the first search.
	Indexes []*fts.Index
}
```

FullTextConfig applies an fts.Policy to the full-text indexes searched
through a connection.

## connection.MigrationConfigurer

```go
type MigrationConfigurer interface {
	MigrationConfig(cfg Config) Config
}
```

MigrationConfigurer is implemented by drivers whose migration pool needs a
configuration the query pools must not have, e.g. MySQL's multiStatements.

## connection.Option

```go
type Option func(*options)
```

options

### connection.WithFullTextPolicy

```go
func WithFullTextPolicy(p fts.Policy) Option
```

WithFullTextPolicy sets the full-text policy of a connection built by
NewRaw (Config.FullText does it for NewConnection); use fts.Trust with a
mocked database.

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

PoolConfig bounds the pool. A zero field takes its DefaultPoolConfig value,
so a zero PoolConfig never means database/sql's unlimited connections and
infinite lifetime. A negative MaxOpenConns or MaxConnLifeTime explicitly
removes that limit; a negative MaxIdleConns keeps no idle connections.

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

### connection.Logged

```go
func Logged(q Querier) Querier
```

Logged returns q logging its statements while LogQueries or PrintQueries
is on.

### connection.QuerierFrom

```go
func QuerierFrom(ctx context.Context, db *sql.DB) Querier
```

QuerierFrom returns db's transaction carried by ctx, or db; it logs its
statements while LogQueries is on.

## connection.Settings

```go
type Settings struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	// SSLMode selects transport security (see the SSL* constants). Empty
	// means verify-full, except on a loopback host or unix socket, where it
	// means disable so local development works without certificates.
	SSLMode string
	// SSLRootCert is the PEM file of the CAs that sign the server
	// certificate (e.g. the RDS bundle); empty uses the system roots.
	SSLRootCert string
	// SSLCert and SSLKey are the PEM client certificate and key for mutual TLS.
	SSLCert string
	SSLKey  string
	// StatementTimeout makes the server cancel statements that run longer
	// (postgres statement_timeout); zero keeps the server default.
	StatementTimeout time.Duration
}
```

Settings holds the structured connection parameters that a Driver assembles
into its driver-specific DSN. It decouples DSN construction from any single
configuration source: gofi's config.Database fills it from the DATABASE_*
environment variables, but callers can build it by hand just as well.

### Settings.CheckSSLMode

```go
func (s Settings) CheckSSLMode(supported ...string) error
```

CheckSSLMode fails when the effective SSL mode is not in supported.

### Settings.EffectiveSSLMode

```go
func (s Settings) EffectiveSSLMode() string
```

EffectiveSSLMode is the SSL mode drivers apply: SSLMode, or its default
when empty (disable on a local host, verify-full elsewhere).

### Settings.Insecure

```go
func (s Settings) Insecure() bool
```

Insecure reports whether the connection may run in plaintext or without
verifying the server identity: any effective mode other than verify-ca and
verify-full, unknown modes included. Production guards should refuse it
unless IsLocal (e.g. a sidecar proxy that owns TLS).

### Settings.IsLocal

```go
func (s Settings) IsLocal() bool
```

IsLocal reports whether Host is empty, a loopback name or address, or a
unix socket directory.

### Settings.Validate

```go
func (s Settings) Validate() error
```

Validate rejects values that no driver can place in a DSN safely: control
characters or whitespace, and URL or DSN delimiters in Host and Name.

## connection.SettingsValidator

```go
type SettingsValidator interface {
	ValidateSettings(s Settings) error
}
```

SettingsValidator is implemented by drivers with their own constraints on
Settings (supported SSL modes, options); BuildDSN calls it.

