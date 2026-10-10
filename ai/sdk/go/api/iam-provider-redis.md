# iam/provider/redis

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/redis"` · SDK gofi/v0.2.4 (310ba1a)

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

## redis.LoginThrottler

```go
type LoginThrottler struct {
	// contains filtered or unexported fields
}
```

LoginThrottler implements port.LoginThrottler with Redis: a sliding window
(sorted set of failure times) per email and per IP, and a lockout key set
once a limit is reached. Shared by every instance.

### redis.NewLoginThrottler

```go
func NewLoginThrottler(client goredis.UniversalClient, cfg ThrottleConfig) *LoginThrottler
```

NewLoginThrottler builds a Redis LoginThrottler.

### LoginThrottler.Allow

```go
func (t *LoginThrottler) Allow(ctx context.Context, a port.LoginAttempt) error
```

Allow returns core.ErrTooManyAttempts while the email or the IP is locked out.

### LoginThrottler.Failure

```go
func (t *LoginThrottler) Failure(ctx context.Context, a port.LoginAttempt) error
```

Failure counts a failed attempt for the email and the IP.

### LoginThrottler.Success

```go
func (t *LoginThrottler) Success(ctx context.Context, a port.LoginAttempt) error
```

Success clears the failures of the email; the IP count and any lockout are kept.

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

### Provider.Client

```go
func (p *Provider) Client() goredis.UniversalClient
```

Client returns the underlying Redis client, to share it with the login
throttler and the ticket store.

### Provider.Get

```go
func (p *Provider) Get(ctx context.Context, sessionID string) (*types.Session, error)
```

Get retrieves a session by ID.

### Provider.ListByUser

```go
func (p *Provider) ListByUser(ctx context.Context, userID string) ([]*types.Session, error)
```

ListByUser returns the active sessions for the given user; expired and
revoked ids are pruned from the user index on the way.

### Provider.Revoke

```go
func (p *Provider) Revoke(ctx context.Context, sessionID string) error
```

Revoke invalidates a specific session by marking it as revoked and updating it in Redis.

### Provider.RevokeAllForUser

```go
func (p *Provider) RevokeAllForUser(ctx context.Context, userID string) error
```

RevokeAllForUser first records the user's cut-off (see RevokedBefore), which
also rejects sessions a concurrent refresh opens, then revokes every indexed
session. Every failure is returned (joined); expired ids leave the index.

### Provider.RevokeIfActive

```go
func (p *Provider) RevokeIfActive(ctx context.Context, sessionID string) (bool, error)
```

RevokeIfActive revokes the session with an optimistic WATCH/MULTI transaction,
so only one of several concurrent refreshes can rotate it.

### Provider.RevokedBefore

```go
func (p *Provider) RevokedBefore(ctx context.Context, userID string) (time.Time, error)
```

RevokedBefore implements port.UserRevocationStore; zero when never set.

### Provider.Save

```go
func (p *Provider) Save(ctx context.Context, session *types.Session) error
```

Save persists the session in Redis with a TTL calculated from ExpiresAt.
The raw RefreshToken field is never serialized — only RefreshTokenHash is persisted.
SessionExtra is stored as is (plain JSON): protect Redis with TLS and ACLs
when it carries external provider tokens.

## redis.ThrottleConfig

```go
type ThrottleConfig struct {
	MaxAttempts   int           // failures per email within Window; default 5
	IPMaxAttempts int           // failures per IP within Window; default 4x MaxAttempts
	Window        time.Duration // sliding window; default 15 min
	Lockout       time.Duration // lockout once a limit is reached; default 15 min
	KeyPrefix     string        // default "iam:throttle:"
}
```

ThrottleConfig configures a LoginThrottler. Zero values use the defaults.

## redis.TicketStore

```go
type TicketStore struct {
	// contains filtered or unexported fields
}
```

TicketStore implements port.TicketStore with SET NX, shared by every instance.

### redis.NewTicketStore

```go
func NewTicketStore(client goredis.UniversalClient, keyPrefix string) *TicketStore
```

NewTicketStore builds a Redis TicketStore; keyPrefix defaults to "iam:ticket:".

### TicketStore.Consume

```go
func (s *TicketStore) Consume(ctx context.Context, jti string, ttl time.Duration) (bool, error)
```

Consume records jti for ttl and reports false when it was already consumed.

