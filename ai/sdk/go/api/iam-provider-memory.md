# iam/provider/memory

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/memory"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package memory implements port.SessionPort in memory.
Two modes are supported: with TTL for development and CI with periodic GC,
and without TTL for unit tests with deterministic, goroutine-free behavior.

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

RevokeAllForUser invalidates all sessions for the given user.

### Provider.RevokeIfActive

```go
func (p *Provider) RevokeIfActive(_ context.Context, sessionID string) (bool, error)
```

RevokeIfActive revokes the session unless it is already revoked.

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

