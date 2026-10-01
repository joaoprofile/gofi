# netx/awssign

`import "github.com/gofi-labs/gofi-sdk-go/netx/awssign"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package awssign signs netx requests with AWS Signature V4 (API Gateway IAM
auth, OpenSearch, Lambda URLs, ...). Credentials resolve through base/cloud/aws,
so IRSA / Pod Identity work without static keys.

## Constantes

```go
const ServiceExecuteAPI = "execute-api"
```

ServiceExecuteAPI is the signing name of API Gateway.

## awssign.Config

```go
type Config struct {
	// Service is the SigV4 signing name; defaults to ServiceExecuteAPI.
	Service string
	AWS     cloudaws.Config
}
```

Config selects the target service and the AWS identity.

## awssign.Signer

```go
type Signer struct {
	// contains filtered or unexported fields
}
```

Signer implements netx.Signature.

### awssign.New

```go
func New(ctx context.Context, cfg Config) (*Signer, error)
```

New resolves credentials and region for cfg.

### awssign.NewWithConfig

```go
func NewWithConfig(awsCfg awssdk.Config, service string) (*Signer, error)
```

NewWithConfig builds a Signer from an existing aws.Config.

### Signer.Sign

```go
func (s *Signer) Sign(req *http.Request, body []byte) (*http.Request, error)
```

Sign adds the SigV4 headers; body must be the exact payload sent.

