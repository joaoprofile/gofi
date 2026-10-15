# gofi/component/session

`import "github.com/joaoprofile/gofi-sdk-go/gofi/component/session"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package session is the gofi component that installs the global session
store (base/session) backed by the driver selected by CACHE_TYPE.

## session.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component installs session.Instance(). Distributed locking comes with the
driver, so there is no separate locker component.

### session.New

```go
func New(cfg ...*basesession.Config) *Component
```

New uses cfg, or basesession.DefaultSessionConfig when it is omitted or nil.
With CACHE_TYPE=redis it reuses the cache component's Redis client, opening
the shared one when the service has no cache component.

### Component.Config

```go
func (c *Component) Config() *basesession.Config
```

Config returns the session configuration in use.

### Component.InsecureTransports

```go
func (c *Component) InsecureTransports(env *environment.Environment) []gofi.InsecureTransport
```

InsecureTransports reports the CACHE_* Redis in plaintext when
CACHE_TYPE=redis (see cache.Component).

### Component.Name

```go
func (c *Component) Name() string
```

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(_ context.Context, rt *gofi.Runtime) error
```

