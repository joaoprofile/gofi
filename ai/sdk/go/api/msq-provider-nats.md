# msq/provider/nats

`import "github.com/gofi-labs/gofi-sdk-go/msq/provider/nats"` · SDK v0.8.2

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

	// Authentication: credentials file (JWT + NKey), token or user/password.
	CredsFile string
	Token     string
	User      string
	Password  string

	// Streams created for subjects without one: Replicas (default 1) and
	// MaxAge, the retention bound (0 = no age limit).
	Replicas int
	MaxAge   time.Duration

	// AckWait is how long an unacked message waits before redelivery (default 30s).
	AckWait time.Duration
}
```

Config configures the connection and the streams created on demand.

