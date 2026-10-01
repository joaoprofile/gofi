# msq/provider/rabbitmq

`import "github.com/gofi-labs/gofi-sdk-go/msq/provider/rabbitmq"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package rabbitmq implements port.Broker for RabbitMQ using AMQP 0-9-1.

## Variáveis

```go
var ErrNotConnected = errors.New("rabbitmq: not connected")
```

ErrNotConnected is returned while the connection is being re-established.

## rabbitmq.Broker

```go
type Broker struct {
	// contains filtered or unexported fields
}
```

Broker implements port.Broker for RabbitMQ.

### rabbitmq.New

```go
func New(conn *Conn, exchange string, opts ...Option) *Broker
```

New creates a Broker bound to the given connection and exchange.

### Broker.Close

```go
func (b *Broker) Close() error
```

Close closes the connection.

### Broker.NewConsumer

```go
func (b *Broker) NewConsumer(cfg types.ConsumeConfig) (port.Consumer, error)
```

### Broker.NewProducer

```go
func (b *Broker) NewProducer() (port.Producer, error)
```

NewProducer opens a channel in confirm mode: SendMessage returns only after
the broker has taken responsibility for the message.

### Broker.Setup

```go
func (b *Broker) Setup(context.Context) error
```

Setup declares the exchange (durable, direct); the default exchange ("")
needs none. gofi's builder calls it during Build.

## rabbitmq.Conn

```go
type Conn struct {
	// contains filtered or unexported fields
}
```

Conn is an AMQP connection that re-dials with backoff when the broker drops
it. Producers and consumers re-open their channels on the new connection.

### rabbitmq.DialURL

```go
func DialURL(url string) (*Conn, error)
```

DialURL establishes a connection to the given AMQP URL (amqp:// or amqps://).
The caller closes it (Broker.Close does; gofi's builder closes the Broker).

### Conn.Close

```go
func (c *Conn) Close() error
```

Close closes the connection and stops reconnecting. It is idempotent.

### Conn.IsConnected

```go
func (c *Conn) IsConnected() bool
```

IsConnected reports whether a live connection is available.

### Conn.Setup

```go
func (c *Conn) Setup(_ context.Context, exchangeName string) error
```

Setup declares a durable direct exchange. Broker.Setup calls it; call it
directly only when using Conn without a Broker.

## rabbitmq.Option

```go
type Option func(*Broker)
```

Option configures a Broker.

### rabbitmq.WithEncoding

```go
func WithEncoding(e types.Encoding) Option
```

WithEncoding selects the wire format producers write; consumers read both.
The default is types.EncodingEnvelope.

