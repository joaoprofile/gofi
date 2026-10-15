# msq/provider/kafka

`import "github.com/joaoprofile/gofi-sdk-go/msq/provider/kafka"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package kafka implements port.Broker for Apache Kafka using the Sarama library.

## Variáveis

```go
var (
	ErrPartialCredentials = errors.New("kafka: SASL needs both User and Password")
	ErrUnknownMechanism   = errors.New("kafka: unknown SASL mechanism (PLAIN, SCRAM-SHA-256 or SCRAM-SHA-512)")
	ErrPlaintextSASL      = errors.New("kafka: SASL PLAIN without TLS sends the password in clear text; enable TLS or set AllowPlaintextSASL")
)
```

Errors returned by New for unsafe or inconsistent configurations.

## kafka.Broker

```go
type Broker struct {
	// contains filtered or unexported fields
}
```

Broker implements port.Broker for Kafka.

### kafka.New

```go
func New(cfg Config) (*Broker, error)
```

New creates a Broker from the given configuration.

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
func (b *Broker) Setup(_ context.Context) error
```

Setup implements port.BrokerSetup. It creates the configured topics
idempotently — topics that already exist are silently skipped.

## kafka.Config

```go
type Config struct {
	Brokers []string
	// User and Password enable SASL; set both or neither.
	User     string
	Password string
	UseTLS   bool
	// TLS is the client TLS configuration (private CA, mTLS, server name);
	// setting it enables TLS. Versions below TLS 1.2 are raised to it.
	TLS *tls.Config `json:"-"`
	// SASLMechanism selects the SASL mechanism: "PLAIN" (default),
	// "SCRAM-SHA-256" or "SCRAM-SHA-512". Managed brokers such as OCI Kafka
	// typically require SCRAM. Empty means PLAIN.
	SASLMechanism string
	// AllowPlaintextSASL permits SASL PLAIN without TLS, which sends the
	// password in clear text. Only for local development.
	AllowPlaintextSASL bool
	// Acks is how many replicas must store a record before a send succeeds:
	// "all" (default, with the idempotent producer), "leader" or "none".
	// Anything but "all" disables idempotence and can lose acknowledged
	// records on a leader failover.
	Acks     string
	ClientID string
	// Topics lists topics to be created idempotently when Setup is called.
	Topics []TopicConfig
}
```

Config configures the Kafka broker. Printing or logging it redacts the
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

## kafka.TopicConfig

```go
type TopicConfig struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	ConfigEntries     map[string]*string
}
```

TopicConfig describes a Kafka topic that should be created during Setup.

