# sqln/migrate

`import "github.com/gofi-labs/gofi-sdk-go/sqln/migrate"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### migrate.RegisterDriver

```go
func RegisterDriver(d Driver)
```

### migrate.Run

```go
func Run(db *sql.DB, driverName string, cfg Config) error
```

Run applies migrations using db; the caller keeps ownership of db.

### migrate.RunAndClose

```go
func RunAndClose(db *sql.DB, driverName string, cfg Config) error
```

RunAndClose applies migrations on a database opened only for them and closes it
afterwards, releasing the connection golang-migrate keeps pinned.

## migrate.Config

```go
type Config struct {
	Path string
	FS   embed.FS
}
```

## migrate.Driver

```go
type Driver interface {
	Name() string
	Instance(db *sql.DB) (database.Driver, error)
}
```

### migrate.GetDriver

```go
func GetDriver(name string) (Driver, bool)
```

