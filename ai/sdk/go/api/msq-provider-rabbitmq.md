# msq/provider/rabbitmq

`import "github.com/joaoprofile/gofi-sdk-go/msq/provider/rabbitmq"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package rabbitmq implements port.Broker for RabbitMQ using AMQP 0-9-1.

## Variáveis

```go
var ErrNotConnected = errors.New("rabbitmq: not connected")
```

ErrNotConnected is returned while the connection is being re-established.

```go
var ErrUnroutable = errors.New("rabbitmq: message unroutable")
```

ErrUnroutable is returned when the broker has no queue bound for the
message's routing key: the message was not stored.

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
the broker has taken responsibility for the message. Messages are published
as mandatory, so one no queue is bound for fails with ErrUnroutable instead
of being dropped: declare the queues (start the consumers) before producing.

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
func DialURL(rawURL string, opts ...DialOption) (*Conn, error)
```

DialURL establishes a connection to the given AMQP URL (amqp:// or amqps://).
Credentials in the URL are sent with SASL PLAIN and never appear in errors
or logs. The caller closes it (Broker.Close does; gofi's builder closes the
Broker).

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

## rabbitmq.DialOption

```go
type DialOption func(*amqp091.Config)
```

DialOption configures DialURL.

### rabbitmq.WithTLSConfig

```go
func WithTLSConfig(c *tls.Config) DialOption
```

WithTLSConfig sets the TLS configuration of an amqps:// connection
(private CA, client certificate, server name). Versions below TLS 1.2 are
raised to it.

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

