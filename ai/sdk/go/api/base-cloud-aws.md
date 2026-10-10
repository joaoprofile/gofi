# base/cloud/aws

`import "github.com/joaoprofile/gofi-sdk-go/base/cloud/aws"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package aws loads AWS credentials for every gofi integration (S3, SQS, RDS,
SigV4 signing). Without explicit keys it uses the default credential chain:
environment, shared config, IRSA / EKS Pod Identity, ECS and EC2 roles.

## Funções

### aws.Load

```go
func Load(ctx context.Context, cfg Config) (awssdk.Config, error)
```

Load resolves an aws.Config for cfg.

## aws.Config

```go
type Config struct {
	Region string
	// Endpoint overrides the service endpoint (LocalStack, MinIO, R2, ...).
	Endpoint string
	// Static credentials; leave empty to use the default credential chain.
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	// Profile selects a shared-config profile.
	Profile string
}
```

Config holds optional overrides; the zero value uses the default chain.

### Config.Format

```go
func (c Config) Format(f fmt.State, verb rune)
```

### Config.GoString

```go
func (c Config) GoString() string
```

### Config.LogValue

```go
func (c Config) LogValue() slog.Value
```

### Config.MarshalJSON

```go
func (c Config) MarshalJSON() ([]byte, error)
```

### Config.String

```go
func (c Config) String() string
```

String, GoString, Format, LogValue and MarshalJSON mask the secret fields.

