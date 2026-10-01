# gofi/component/cache

`import "github.com/gofi-labs/gofi-sdk-go/gofi/component/cache"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package cache is the gofi component for the shared Redis client used by the
sqln query cache and by the session component.

## Funções

### cache.Configure

```go
func Configure(env *environment.Environment)
```

Configure wires the sqln Redis cache from CACHE_* and namespaces keys
with APP_NAME. Call it at startup before the first cache access.

### cache.Shared

```go
func Shared(rt *gofi.Runtime) (redis.UniversalClient, error)
```

Shared returns the Redis client of this Build: the one injected with
FromClient or, otherwise, the sqln shared client, pinged once and closed on
Shutdown. Components that need Redis (session) call it from Start.

## cache.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component opens the shared Redis client.

### cache.FromClient

```go
func FromClient(client redis.UniversalClient) *Component
```

FromClient uses a client created by the caller, who owns and closes it. The
session component reuses it; the sqln query cache keeps its own client.

### cache.New

```go
func New() *Component
```

New opens the shared Redis client from CACHE_* during Build; an unreachable
Redis fails Build instead of degrading silently.

### Component.Client

```go
func (c *Component) Client() redis.UniversalClient
```

Client returns the Redis client; nil before Build.

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

