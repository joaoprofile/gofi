# gofi/component/database

`import "github.com/gofi-labs/gofi-sdk-go/gofi/component/database"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package database is the gofi component for the SQL database (sqln).

It links no SQL driver: blank-import the driver package for DATABASE_DRIVER
(postgres by default) in main, or Build fails naming the missing import:

	import _ "github.com/gofi-labs/gofi-sdk-go/sqln/driver/postgres"

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
driver is not registered.

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
func New() *Component
```

New opens the database from DATABASE_* (see ConfigFromEnv) during Build.
With DATABASE_MIGRATION=true it runs the migrations in .migrations first.

### Component.DB

```go
func (c *Component) DB() *sql.DB
```

DB returns the primary pool; nil before Build.

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

