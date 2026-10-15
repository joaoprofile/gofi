# gofi/component/database

`import "github.com/joaoprofile/gofi-sdk-go/gofi/component/database"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package database is the gofi component for the SQL database (sqln).

It links no SQL driver: blank-import the driver package for DATABASE_DRIVER
(postgres by default) in main, or Build fails naming the missing import:

	import _ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres"

## Funções

### database.ConfigFromEnv

```go
func ConfigFromEnv(env *environment.Environment) (connection.Config, error)
```

ConfigFromEnv builds a connection.Config from the DATABASE_* environment variables.

It is driver-agnostic: the DSN is assembled by whichever driver is registered
for DATABASE_DRIVER (blank-import its sqln/driver/<name> package to register
it), so adding a new database requires no change here. The driver defaults to
postgres when DATABASE_DRIVER is empty. Returns an error when the requested
driver is not registered or the settings are invalid (connection.BuildDSN).

## database.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component opens the database and publishes it as the global sqln connection.

### database.FromDB

```go
func FromDB(db *sql.DB) *Component
```

FromDB uses a *sql.DB opened by the caller, who owns and closes it. It gets
the same health check and pool metrics, but is not made the global sqln
connection.

### database.New

```go
func New(opts ...Option) *Component
```

New opens the database from DATABASE_* (see ConfigFromEnv) during Build.
With DATABASE_MIGRATION=true it runs the migrations in .migrations first,
then checks the full-text indexes declared with fts.MustIndex in the
packages linked into the binary (see WithFullText): a missing or outdated
index fails the startup. DATABASE_FULLTEXT_POLICY selects verify
(default), ensure (creates missing indexes; refused in stage and prod) or
trust (no check). DATABASE_LOG_QUERIES=true logs every SQL statement
sqln runs, with its arguments and duration, and
DATABASE_LOG_QUERIES_PRETTY=true prints them indented, one argument per
line, to stderr (both refused in stage and prod).

### Component.DB

```go
func (c *Component) DB() *sql.DB
```

DB returns the primary pool; nil before Build.

### Component.InsecureTransports

```go
func (c *Component) InsecureTransports(env *environment.Environment) []gofi.InsecureTransport
```

InsecureTransports reports a primary or replica on a non-local host whose
DATABASE_SSL_MODE does not verify the server. Injected pools are not checked.

### Component.Name

```go
func (c *Component) Name() string
```

### Component.ReadDB

```go
func (c *Component) ReadDB() *sql.DB
```

ReadDB returns the read replica pool, or the primary when there is none.

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(_ context.Context, rt *gofi.Runtime) error
```

## database.Option

```go
type Option func(*Component)
```

Option configures a Component built by New.

### database.WithFullText

```go
func WithFullText(indexes ...*fts.Index) Option
```

WithFullText checks exactly these full-text indexes at startup instead of
every index declared with fts.MustIndex: use it when the service talks to
several databases. With no index, nothing is checked at startup.

