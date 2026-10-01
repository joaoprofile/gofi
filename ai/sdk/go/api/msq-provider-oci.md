# msq/provider/oci

`import "github.com/gofi-labs/gofi-sdk-go/msq/provider/oci"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## oci.Broker

```go
type Broker struct {
	// contains filtered or unexported fields
}
```

Broker implements port.Broker for OCI Queue.

### oci.New

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

## oci.Config

```go
type Config struct {
	// Credentials selects the principal (API key, instance, resource or
	// workload identity) and region, shared with other OCI integrations.
	Credentials cloudoci.Config
	// QueueURL is the queue's messages endpoint; it defaults to the
	// region's cell-1 endpoint.
	QueueURL string
}
```

Config configures the OCI Queue broker.

