# base/session

`import "github.com/gofi-labs/gofi-sdk-go/base/session"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	StateProcessing = "processing"
	StateDone       = "done"
	StateFailed     = "failed"
)
```

## Variáveis

```go
var (
	ErrAlreadyLocked  = errors.New("resource is already locked")
	ErrNotLocked      = errors.New("resource is not locked")
	ErrInvalidKey     = errors.New("invalid key for locking")
	ErrEncodingFailed = errors.New("failed to encode lock object")
)
```

## Funções

### session.GenerateSessionID

```go
func GenerateSessionID() string
```

### session.NewKey

```go
func NewKey(prefix string, parts ...any) string
```

## session.Config

```go
type Config struct {
	TTL    time.Duration
	Prefix string
}
```

Config holds the session manager configuration.

### session.DefaultSessionConfig

```go
func DefaultSessionConfig() *Config
```

DefaultSessionConfig returns a Config with sensible defaults
(10-minute TTL, "session" prefix).

## session.DistributedLocker

```go
type DistributedLocker interface {
	TryLock(ctx context.Context, key string) (bool, error)
	Unlock(ctx context.Context, key string) error
	IsLocked(ctx context.Context, key string) (bool, error)
	WithLock(ctx context.Context, key string, fn func() error) (bool, error)
}
```

DistributedLocker is the contract for distributed locking.
*Session implements this interface, so it can be injected wherever a DistributedLocker is expected.

## session.Driver

```go
type Driver interface {
	Save(ctx context.Context, key string, entry *Entry) error
	Get(ctx context.Context, key string) (*Entry, error)
	Delete(ctx context.Context, key string) error
	ScanAll(ctx context.Context, prefix string) ([]string, error)
	CleanExpired(ctx context.Context, prefix string) error
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
	IsLocked(ctx context.Context, key string) (bool, error)
}
```

Driver is the pluggable storage backend contract.
Implement this interface to add new session backends (Redis, OCI, DynamoDB, in-memory, etc.).

### session.NewRedisDriver

```go
func NewRedisDriver(client redis.UniversalClient, ttl time.Duration) Driver
```

NewRedisDriver returns a Driver backed by the given Redis client.
ttl is used as the fallback when an Entry has already expired at save time
or when acquiring a lock with ttl=0.

## session.Entry

```go
type Entry struct {
	ID        string      `json:"id"`
	Data      SessionData `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}
```

Entry is the session record stored by a Driver.

### Entry.Extend

```go
func (e *Entry) Extend(d time.Duration)
```

### Entry.Get

```go
func (e *Entry) Get(key string) any
```

### Entry.IsExpired

```go
func (e *Entry) IsExpired() bool
```

### Entry.Set

```go
func (e *Entry) Set(key string, value any) *Entry
```

### Entry.TTL

```go
func (e *Entry) TTL() time.Duration
```

## session.LockEntry

```go
type LockEntry struct {
	Key   string `json:"key"`
	State string `json:"state"`
}
```

LockEntry is the value stored in the backend when a lock is held.

## session.RedisDriver

```go
type RedisDriver struct {
	// contains filtered or unexported fields
}
```

RedisDriver implements Driver using a Redis backend.
Use NewRedisDriver to construct one; it satisfies the Driver interface
so it can be passed directly to New.

### RedisDriver.AcquireLock

```go
func (r *RedisDriver) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
```

### RedisDriver.AcquireLockToken

```go
func (r *RedisDriver) AcquireLockToken(ctx context.Context, key string, ttl time.Duration) (string, bool, error)
```

### RedisDriver.CleanExpired

```go
func (r *RedisDriver) CleanExpired(ctx context.Context, prefix string) error
```

### RedisDriver.Delete

```go
func (r *RedisDriver) Delete(ctx context.Context, key string) error
```

### RedisDriver.Get

```go
func (r *RedisDriver) Get(ctx context.Context, key string) (*Entry, error)
```

### RedisDriver.IsLocked

```go
func (r *RedisDriver) IsLocked(ctx context.Context, key string) (bool, error)
```

### RedisDriver.ReleaseLock

```go
func (r *RedisDriver) ReleaseLock(ctx context.Context, key string) error
```

### RedisDriver.ReleaseLockToken

```go
func (r *RedisDriver) ReleaseLockToken(ctx context.Context, key, token string) error
```

### RedisDriver.Save

```go
func (r *RedisDriver) Save(ctx context.Context, key string, entry *Entry) error
```

### RedisDriver.ScanAll

```go
func (r *RedisDriver) ScanAll(ctx context.Context, prefix string) ([]string, error)
```

## session.Session

```go
type Session struct {
	Prefix string
	TTL    time.Duration
	// contains filtered or unexported fields
}
```

Session is the unified session manager and distributed locker.
It delegates all storage operations to a pluggable Driver, making
it easy to swap backends (Redis, OCI, in-memory, etc.) without
changing any consumer code.

*Session implements DistributedLocker, so it can be injected as a
lock provider wherever that interface is expected.

### session.Instance

```go
func Instance() *Session
```

Instance returns the singleton Session, or nil if New has not been called.

### session.New

```go
func New(driver Driver, cfg *Config) *Session
```

New initialises the Session singleton. Subsequent calls are no-ops;
the first call wins. Pass a different Driver to switch backends.

### Session.CleanExpired

```go
func (s *Session) CleanExpired(ctx context.Context, prefix string) error
```

CleanExpired removes all expired entries whose keys match prefix.

### Session.CreateOrGet

```go
func (s *Session) CreateOrGet(ctx context.Context, key string, data SessionData) (*Entry, error)
```

CreateOrGet returns an existing, non-expired entry for key, or creates and
stores a new one from data — all under a distributed lock.

### Session.Delete

```go
func (s *Session) Delete(ctx context.Context, key string) error
```

Delete removes an entry by key.

### Session.Force

```go
func (s *Session) Force(ctx context.Context, key string, data SessionData) (*Entry, error)
```

Force overwrites any existing entry under key with fresh data.

### Session.Get

```go
func (s *Session) Get(ctx context.Context, key string) (*Entry, error)
```

Get retrieves an entry by key without creating one (no lock needed).

### Session.IsLocked

```go
func (s *Session) IsLocked(ctx context.Context, key string) (bool, error)
```

IsLocked reports whether key is currently locked.

### Session.TryLock

```go
func (s *Session) TryLock(ctx context.Context, key string) (bool, error)
```

TryLock attempts to acquire a distributed lock for key.
The key is used as-is; callers should use NewKey to build
collision-resistant identifiers.

### Session.Unlock

```go
func (s *Session) Unlock(ctx context.Context, key string) error
```

Unlock releases the distributed lock for key.

### Session.WithLock

```go
func (s *Session) WithLock(ctx context.Context, key string, fn func() error) (bool, error)
```

WithLock acquires the lock, runs fn, then releases it.
Returns (false, nil) if the key is already locked.
Returns (true, err) if fn returned an error (lock is still released).

## session.SessionData

```go
type SessionData map[string]any
```

SessionData holds the arbitrary key-value payload of an entry.

## session.TokenLocker

```go
type TokenLocker interface {
	AcquireLockToken(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error)
	ReleaseLockToken(ctx context.Context, key, token string) error
}
```

TokenLocker is optionally implemented by a Driver so that a lock can only be
released by the holder that acquired it, even after its TTL expired.

