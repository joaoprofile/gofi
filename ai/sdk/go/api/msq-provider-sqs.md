# msq/provider/sqs

`import "github.com/joaoprofile/gofi-sdk-go/msq/provider/sqs"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package sqs implements the msq broker on Amazon SQS (aws-sdk-go-v2).

## sqs.Broker

```go
type Broker struct {
	// contains filtered or unexported fields
}
```

Broker implements port.Broker for SQS. Queue URLs are resolved once per name.

### sqs.New

```go
func New(ctx context.Context, cfg Config) (*Broker, error)
```

New builds a Broker from cfg.

### sqs.NewWithConfig

```go
func NewWithConfig(awsCfg awssdk.Config) *Broker
```

NewWithConfig builds a Broker from an existing aws.Config.

### Broker.NewConsumer

```go
func (b *Broker) NewConsumer(cfg types.ConsumeConfig) (port.Consumer, error)
```

### Broker.NewProducer

```go
func (b *Broker) NewProducer() (port.Producer, error)
```

## sqs.Config

```go
type Config struct {
	AWS cloudaws.Config
}
```

Config selects the AWS identity; the zero value uses the default credential
chain (IRSA, EKS Pod Identity, instance profile, environment).

