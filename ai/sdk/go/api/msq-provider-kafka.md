# msq/provider/kafka

`import "github.com/joaoprofile/gofi-sdk-go/msq/provider/kafka"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package kafka implements port.Broker for Apache Kafka using the Sarama library.

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
	Brokers  []string
	User     string
	Password string
	UseTLS   bool
	// SASLMechanism selects the SASL mechanism: "PLAIN" (default),
	// "SCRAM-SHA-256" or "SCRAM-SHA-512". Managed brokers such as OCI Kafka
	// typically require SCRAM. Empty means PLAIN.
	SASLMechanism string
	ClientID      string
	// Topics lists topics to be created idempotently when Setup is called.
	Topics []TopicConfig
}
```

Config configures the Kafka broker.

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

