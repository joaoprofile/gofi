# msq/provider/nats

`import "github.com/joaoprofile/gofi-sdk-go/msq/provider/nats"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package nats implements port.Broker for NATS JetStream: persistent
subjects, durable consumer groups, explicit acks and redelivery. Messages
use CloudEvents binary mode and the message Id as the JetStream
de-duplication id.

## nats.Broker

```go
type Broker struct {
	// contains filtered or unexported fields
}
```

Broker implements port.Broker and port.BrokerSetup.

### nats.New

```go
func New(cfg Config) (*Broker, error)
```

New connects; it reconnects forever in the background.

### Broker.Close

```go
func (b *Broker) Close() error
```

Close drains the connection: pending publishes and acks are flushed.

### Broker.NewConsumer

```go
func (b *Broker) NewConsumer(cfg types.ConsumeConfig) (port.Consumer, error)
```

### Broker.NewProducer

```go
func (b *Broker) NewProducer() (port.Producer, error)
```

### Broker.Setup

```go
func (b *Broker) Setup(ctx context.Context) error
```

Setup checks that JetStream is enabled on the server.

## nats.Config

```go
type Config struct {
	// URL is one or more comma-separated server URLs (nats://host:4222).
	URL  string
	Name string // client name shown in server monitoring

	// TLS is the client TLS configuration (private CA, mTLS, server name);
	// setting it requires TLS. Versions below TLS 1.2 are raised to it.
	TLS *tls.Config `json:"-"`

	// Authentication: credentials file (JWT + NKey), token or user/password.
	CredsFile string
	Token     string
	User      string
	Password  string

	// Streams created for subjects without one: Replicas (default 1) and
	// MaxAge, the retention bound (0 = no age limit).
	Replicas int
	MaxAge   time.Duration

	// AckWait is how long an unacked message waits before redelivery when
	// ConsumeConfig.VisibilityTimeout is zero (default 30s). It applies when
	// the durable consumer is created; the lease is renewed while handling.
	AckWait time.Duration
}
```

Config configures the connection and the streams created on demand.
Printing or logging it redacts the token and password.

### Config.GoString

```go
func (c Config) GoString() string
```

GoString implements fmt.GoStringer with the secrets redacted.

### Config.LogValue

```go
func (c Config) LogValue() slog.Value
```

LogValue implements slog.LogValuer with the secrets redacted.

### Config.MarshalJSON

```go
func (c Config) MarshalJSON() ([]byte, error)
```

MarshalJSON encodes the configuration with the secrets redacted.

### Config.String

```go
func (c Config) String() string
```

String implements fmt.Stringer with the secrets redacted.

