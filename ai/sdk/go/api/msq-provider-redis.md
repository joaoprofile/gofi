# msq/provider/redis

`import "github.com/gofi-labs/gofi-sdk-go/msq/provider/redis"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## redis.Broker

```go
type Broker struct {
	// contains filtered or unexported fields
}
```

Broker implements port.Broker using Redis Pub/Sub or Streams.

### redis.New

```go
func New(cfg Config) *Broker
```

New creates a Broker from configuration.

### redis.NewWithClient

```go
func NewWithClient(client goredis.UniversalClient, cfg ...Config) *Broker
```

NewWithClient creates a Broker reusing an existing Redis client, e.g. the
cache pool. Only the Mode, StreamMaxLen and ClaimIdle fields of cfg apply.

### Broker.NewConsumer

```go
func (b *Broker) NewConsumer(cfg types.ConsumeConfig) (port.Consumer, error)
```

NewConsumer returns a consumer of cfg.Topic for the configured mode.

### Broker.NewProducer

```go
func (b *Broker) NewProducer() (port.Producer, error)
```

NewProducer returns a producer for the configured mode.

## redis.Config

```go
type Config struct {
	// Mode defaults to ModePubSub.
	Mode Mode
	// StreamMaxLen caps each stream approximately (XADD MAXLEN ~); 0 keeps all.
	StreamMaxLen int64
	// ClaimIdle is how long an unacked stream message waits before it is
	// redelivered (default 1m).
	ClaimIdle time.Duration

	// Standalone mode.
	Addr     string
	Password string
	DB       int

	// Cluster mode (takes precedence over Addr when set).
	ClusterAddrs []string

	// TLS
	TLSEnabled bool

	// Connection pool.
	PoolSize     int
	MinIdleConns int
}
```

Config configures the Redis broker.

## redis.Mode

```go
type Mode string
```

Mode selects the Redis messaging primitive.

```go
const (
	// ModePubSub uses Pub/Sub: at-most-once, subscribers must be connected.
	ModePubSub Mode = "pubsub"
	// ModeStreams uses Streams with consumer groups: at-least-once,
	// messages wait for consumers and Nacks are redelivered.
	ModeStreams Mode = "streams"
)
```

