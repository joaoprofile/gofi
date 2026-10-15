# msq/provider/redis

`import "github.com/joaoprofile/gofi-sdk-go/msq/provider/redis"` · SDK gofi/v0.2.6 (3df9d20)

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
	// redelivered (default 1m); ConsumeConfig.VisibilityTimeout overrides it.
	ClaimIdle time.Duration

	// Standalone mode.
	Addr     string
	Password string
	DB       int

	// Cluster mode (takes precedence over Addr when set).
	ClusterAddrs []string

	// TLS
	TLSEnabled bool
	// TLS is the client TLS configuration (private CA, mTLS, server name);
	// setting it enables TLS. Versions below TLS 1.2 are raised to it.
	TLS *tls.Config `json:"-"`

	// Connection pool.
	PoolSize     int
	MinIdleConns int
}
```

Config configures the Redis broker. Printing or logging it redacts the
password.

### Config.GoString

```go
func (c Config) GoString() string
```

GoString implements fmt.GoStringer with the password redacted.

### Config.LogValue

```go
func (c Config) LogValue() slog.Value
```

LogValue implements slog.LogValuer with the password redacted.

### Config.MarshalJSON

```go
func (c Config) MarshalJSON() ([]byte, error)
```

MarshalJSON encodes the configuration with the password redacted.

### Config.String

```go
func (c Config) String() string
```

String implements fmt.Stringer with the password redacted.

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

