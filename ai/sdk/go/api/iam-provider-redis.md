# iam/provider/redis

`import "github.com/gofi-labs/gofi-sdk-go/iam/provider/redis"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package redis implements port.SessionPort using Redis.
Recommended for production: distributed, native TTL, and instant cross-instance revocation.

## redis.Config

```go
type Config struct {
	// Standalone mode.
	Addr     string
	Password string
	DB       int

	// Cluster mode.
	ClusterAddrs []string

	KeyPrefix string // default: "iam:session:"

	TLSEnabled bool

	// Connection pool settings.
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
```

Config configures the Redis session provider.

## redis.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.SessionPort with Redis.

### redis.NewProvider

```go
func NewProvider(cfg Config) *Provider
```

NewProvider builds a Redis Provider with the given configuration.

### redis.NewProviderWithClient

```go
func NewProviderWithClient(client goredis.UniversalClient, keyPrefix string) *Provider
```

NewProviderWithClient builds a Provider using an existing Redis client.
Useful for tests using miniredis or to reuse an existing connection.

### Provider.Get

```go
func (p *Provider) Get(ctx context.Context, sessionID string) (*types.Session, error)
```

Get retrieves a session by ID.

### Provider.ListByUser

```go
func (p *Provider) ListByUser(ctx context.Context, userID string) ([]*types.Session, error)
```

ListByUser returns the active sessions for the given user.

### Provider.Revoke

```go
func (p *Provider) Revoke(ctx context.Context, sessionID string) error
```

Revoke invalidates a specific session by marking it as revoked and updating it in Redis.

### Provider.RevokeAllForUser

```go
func (p *Provider) RevokeAllForUser(ctx context.Context, userID string) error
```

RevokeAllForUser invalidates all sessions for the given user.

### Provider.RevokeIfActive

```go
func (p *Provider) RevokeIfActive(ctx context.Context, sessionID string) (bool, error)
```

RevokeIfActive revokes the session with an optimistic WATCH/MULTI transaction,
so only one of several concurrent refreshes can rotate it.

### Provider.Save

```go
func (p *Provider) Save(ctx context.Context, session *types.Session) error
```

Save persists the session in Redis with a TTL calculated from ExpiresAt.
The raw RefreshToken field is never serialized — only RefreshTokenHash is persisted.

