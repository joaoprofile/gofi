# base/secrets/awssm

`import "github.com/joaoprofile/gofi-sdk-go/base/secrets/awssm"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package awssm resolves secret://awssm/<name-or-arn>[#key] from AWS Secrets
Manager. Importing it registers the provider; identity and region come from
the AWS default chain (AWS_REGION, IRSA / EKS Pod Identity, roles).

## Constantes

```go
const Provider = "awssm"
```

Provider is the reference provider name.

## awssm.Store

```go
type Store struct {
	// contains filtered or unexported fields
}
```

Store reads secrets from AWS Secrets Manager.

### awssm.New

```go
func New(ctx context.Context, cfg cloudaws.Config) (*Store, error)
```

New resolves the AWS identity for cfg (zero value: default chain).

### awssm.NewWithConfig

```go
func NewWithConfig(awsCfg awssdk.Config) *Store
```

NewWithConfig builds a Store from an existing aws.Config.

### Store.Get

```go
func (s *Store) Get(ctx context.Context, name string) (string, error)
```

Get returns the current version of the secret (string or binary).

