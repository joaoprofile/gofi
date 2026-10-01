# sqln/cache

`import "github.com/joaoprofile/gofi-sdk-go/sqln/cache"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### cache.Close

```go
func Close() error
```

Close waits for work tracked by observer.GetWaitGroup, then closes the
client created by InstanceRedis; injected clients belong to the caller.
It is idempotent.

### cache.Configure

```go
func Configure(c Config)
```

Configure sets the package-level cache configuration. Call it before the
first cache access (NewCacheRedis / InstanceRedis / any Cache operation).

### cache.InstanceRedis

```go
func InstanceRedis() redis.UniversalClient
```

InstanceRedis returns the shared client, creating it on first use. After
Close it keeps returning the closed client (commands fail with
redis.ErrClosed) instead of silently dialing again.

### cache.NewCacheRedis

```go
func NewCacheRedis()
```

NewCacheRedis creates the shared client and logs when Redis is unreachable.
Prefer Ping, which returns the error.

### cache.Ping

```go
func Ping(ctx context.Context) error
```

Ping checks the shared client, creating it when needed.

### cache.UseClient

```go
func UseClient(client redis.UniversalClient)
```

UseClient makes the cache reuse an existing Redis client instead of dialing
its own; the caller keeps ownership of the client. nil resets the cache.

## cache.Cache

```go
type Cache[T any] struct {
	// contains filtered or unexported fields
}
```

### cache.NewCache

```go
func NewCache[T any](name string, ttl time.Duration) *Cache[T]
```

### Cache.Del

```go
func (c *Cache[T]) Del(ctx context.Context) error
```

### Cache.Get

```go
func (c *Cache[T]) Get(ctx context.Context, dest any) (bool, error)
```

Get unmarshals the cached entry into dest (a non-nil pointer). Returns
(true, nil) on hit, (false, nil) on miss, (false, err) on infra/unmarshal
error. Use when the cached shape isn't T or []T — e.g. a *Page[T] from a
paginated query.

### Cache.GetKeyed

```go
func (c *Cache[T]) GetKeyed(ctx context.Context, key string, dest any) (bool, error)
```

GetKeyed reads the entry stored for key (e.g. a query hash) into dest.
Returns (true, nil) on hit and (false, nil) on miss.

### Cache.List

```go
func (c *Cache[T]) List(ctx context.Context) ([]T, error)
```

### Cache.Set

```go
func (c *Cache[T]) Set(ctx context.Context, data any) error
```

### Cache.SetKeyed

```go
func (c *Cache[T]) SetKeyed(ctx context.Context, key string, data any) error
```

SetKeyed stores data under key; Del removes it together with the base entry.

### Cache.UniqueResult

```go
func (c *Cache[T]) UniqueResult(ctx context.Context) (*T, error)
```

### Cache.WithClient

```go
func (c *Cache[T]) WithClient(client redis.UniversalClient) *Cache[T]
```

WithClient points this cache at its own Redis client instead of the shared
one; the caller keeps ownership of the client.

## cache.Config

```go
type Config struct {
	URI      string
	Password string
	TLS      bool // enables TLS 1.2+ to the Redis server
	// Prefix namespaces every key as "<Prefix>::<name>".
	Prefix string
}
```

Config holds the Redis connection settings and key namespace for the cache.
gofi's cache and database components populate it from CACHE_* and APP_NAME; tests call
Configure directly. Set it before the first cache access.

