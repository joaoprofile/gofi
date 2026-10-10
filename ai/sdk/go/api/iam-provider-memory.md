# iam/provider/memory

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/memory"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package memory implements port.SessionPort in memory.
Two modes are supported: with TTL for development and CI with periodic GC,
and without TTL for unit tests with deterministic, goroutine-free behavior.

## memory.LoginThrottler

```go
type LoginThrottler struct {
	// contains filtered or unexported fields
}
```

LoginThrottler implements port.LoginThrottler in memory with a sliding
window per email and per IP. Single instance only; use provider/redis with
several instances.

### memory.NewLoginThrottler

```go
func NewLoginThrottler(cfg ThrottleConfig) *LoginThrottler
```

NewLoginThrottler builds an in-memory LoginThrottler.

### LoginThrottler.Allow

```go
func (t *LoginThrottler) Allow(_ context.Context, a port.LoginAttempt) error
```

Allow returns core.ErrTooManyAttempts while the email or the IP is locked out.

### LoginThrottler.Failure

```go
func (t *LoginThrottler) Failure(_ context.Context, a port.LoginAttempt) error
```

Failure counts a failed attempt and locks the key once its limit is reached.

### LoginThrottler.Success

```go
func (t *LoginThrottler) Success(_ context.Context, a port.LoginAttempt) error
```

Success clears the failures of the email; the IP count is kept.

## memory.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.SessionPort in memory.

### memory.NewProvider

```go
func NewProvider() *Provider
```

NewProvider creates an in-memory Provider with TTL and periodic GC.
Suitable for development and ephemeral environments.
Not suitable for multiple instances as it is not distributed.

### memory.NewTestProvider

```go
func NewTestProvider() *Provider
```

NewTestProvider creates an in-memory Provider without TTL and without goroutines.
Intended exclusively for unit tests.

### Provider.Get

```go
func (p *Provider) Get(_ context.Context, sessionID string) (*types.Session, error)
```

Get retrieves a session by ID. Checks TTL if the provider was created with TTL enabled.

### Provider.ListByUser

```go
func (p *Provider) ListByUser(_ context.Context, userID string) ([]*types.Session, error)
```

ListByUser returns the active sessions for the given user.

### Provider.Revoke

```go
func (p *Provider) Revoke(_ context.Context, sessionID string) error
```

Revoke invalidates a session by marking it as revoked.

### Provider.RevokeAllForUser

```go
func (p *Provider) RevokeAllForUser(_ context.Context, userID string) error
```

RevokeAllForUser invalidates all sessions for the given user and records the
cut-off (see RevokedBefore) that also covers sessions a concurrent refresh opens.

### Provider.RevokeIfActive

```go
func (p *Provider) RevokeIfActive(_ context.Context, sessionID string) (bool, error)
```

RevokeIfActive revokes the session unless it is already revoked.

### Provider.RevokedBefore

```go
func (p *Provider) RevokedBefore(_ context.Context, userID string) (time.Time, error)
```

RevokedBefore implements port.UserRevocationStore; zero when never set.

### Provider.Save

```go
func (p *Provider) Save(_ context.Context, session *types.Session) error
```

Save persists the session in memory. The raw RefreshToken is never stored.

### Provider.Stop

```go
func (p *Provider) Stop()
```

Stop terminates the GC goroutine. Call in a defer when using NewProvider in integration tests.

## memory.ThrottleConfig

```go
type ThrottleConfig struct {
	MaxAttempts   int           // failures per email within Window; default 5
	IPMaxAttempts int           // failures per IP within Window; default 4x MaxAttempts
	Window        time.Duration // sliding window; default 15 min
	Lockout       time.Duration // lockout once a limit is reached; default 15 min
}
```

ThrottleConfig configures a LoginThrottler. Zero values use the defaults.

## memory.TicketStore

```go
type TicketStore struct {
	// contains filtered or unexported fields
}
```

TicketStore implements port.TicketStore in memory. Single instance only.

### memory.NewTicketStore

```go
func NewTicketStore() *TicketStore
```

NewTicketStore builds an in-memory TicketStore.

### TicketStore.Consume

```go
func (s *TicketStore) Consume(_ context.Context, jti string, ttl time.Duration) (bool, error)
```

Consume records jti for ttl and reports false when it was already consumed.

